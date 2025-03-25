// $env:GOOS = "linux"
// $env:GOARCH = "amd64"
// go build -o main-linux

// $env:GOOS = $null
// $env:GOARCH = $null
// go build -o main-win.exe

package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/adshao/go-binance/v2/futures"
)

var (
	client            *futures.Client
	clients           []*futures.Client
	httpClient        *http.Client
	symbolsInfo       []futures.Symbol
	symbolsInfoString []string
	listenKey         string
	err               error
)

// 初始化
func init() {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	// 代理
	if config.Proxy != "" {
		proxyURL, err := url.Parse(config.Proxy)
		if err != nil {
			log.Fatal(err)
		}
		transport.Proxy = http.ProxyURL(proxyURL)
		futures.SetWsProxyUrl(config.Proxy)
	}
	httpClient = &http.Client{Transport: transport}
	// 配置账号
	client = futures.NewClient(config.APIKey, config.SecretKey)
	client.HTTPClient = httpClient
	for _, account := range config.Accounts {
		if account.Switch {
			clients = append(clients, futures.NewClient(account.APIKey, account.SecretKey))
			clients[len(clients)-1].HTTPClient = httpClient
		}
	}
	// 时间偏移
	_, err = client.NewSetServerTimeService().Do(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	for _, c := range clients {
		_, err = c.NewSetServerTimeService().Do(context.Background())
		if err != nil {
			log.Fatal(err)
		}
	}
	// listenKey
	listenKey, err = client.NewStartUserStreamService().Do(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	// 获取交易信息
	info, err := client.NewExchangeInfoService().Do(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	// 赛选币种
	for _, s := range info.Symbols {
		if s.QuoteAsset == "USDT" && s.ContractType == "PERPETUAL" && s.Status == "TRADING" {
			symbolsInfo = append(symbolsInfo, s)
			symbolsInfoString = append(symbolsInfoString, s.BaseAsset)
		}
	}
	if config.Debug {
		log.Println(symbolsInfo)
		log.Println(symbolsInfoString)
	}
}

func main() {
	// 启动ws
	go wsUserGo()
	go riskGo()

	sigC := make(chan os.Signal, 1)
	signal.Notify(sigC, os.Interrupt, syscall.SIGTERM)
	<-sigC
	if config.Debug {
		log.Println("收到退出信号，正在关闭...")
	}
}

func wsUserGo() {
	retryCount := 0
	for {
		doneC, _, err := futures.WsUserDataServe(listenKey, signalHandler, func(err error) {
			time.Sleep(time.Duration(2<<retryCount) * time.Second)
		})
		if err != nil {
			if config.Debug {
				log.Println("WsUserGo 重试中...")
			}
			time.Sleep(time.Second * 5) // 初始等待 5s 再重试
			continue
		}
		retryCount = 0
		log.Println("开始监听")
		if doneC != nil {
			<-doneC
		} else {
			log.Println("Error: doneC is nil")
		}
	}
}

// 信号处理
func signalHandler(event *futures.WsUserDataEvent) {
	if event == nil {
		log.Println("Warning: event is nil")
		return
	}
	//取下单信号
	if event.Event == "ORDER_TRADE_UPDATE" {
		dataOrder := event.OrderTradeUpdate
		if dataOrder.ExecutionType == "TRADE" && dataOrder.Status == "FILLED" {
			if config.Debug {
				//dataOrder转为json格式然后log
				jsonData, err := json.Marshal(dataOrder)
				if err == nil {
					log.Println(string(jsonData))
				}
			}
			orderMapping := map[bool]map[futures.SideType]int{
				false: {"BUY": 1, "SELL": 3},
				true:  {"SELL": 2, "BUY": 4},
			}
			orderMapintSide := map[int]string{
				1: "多下单",
				2: "多平单",
				3: "空下单",
				4: "空平单",
			}
			// 1 多下单 2 多平单 3 空下单 4 空平单
			orderStatus := orderMapping[dataOrder.IsReduceOnly][dataOrder.Side]
			cFree, err := getBalance(client)
			if err != nil {
				log.Println(err)
				return
			}
			if cFree <= 0 {
				log.Println("资金为0")
				return
			}
			// 成交u
			accumulatedFilledQty, err := strconv.ParseFloat(dataOrder.AccumulatedFilledQty, 64)
			if err != nil {
				return
			}
			//
			lastFilledPrice, err := strconv.ParseFloat(dataOrder.LastFilledPrice, 64)
			if err != nil {
				return
			}
			// 订单 与账户的比例
			costRatio := accumulatedFilledQty * lastFilledPrice / cFree

			if config.Debug {
				log.Printf("当前资金：%f", cFree)
				log.Printf("订单状态：%s", orderMapintSide[orderStatus])
				log.Printf("成本比例：%f", costRatio)
				log.Printf("订单价格：%f", lastFilledPrice)
				log.Printf("订单数量：%f", accumulatedFilledQty)
			}
			// 订单数量超出
			if config.IsFloatLoss && costRatio > config.FloatLoss && (orderStatus == 1 || orderStatus == 3) {
				removeCostRatio := costRatio - config.FloatLoss
				if config.Debug {
					log.Printf("规划订单比例：%f", config.FloatLoss)
					log.Printf("超出规划比例：%f", removeCostRatio)
					log.Printf("剔除数量：%f", (1-(removeCostRatio/costRatio))*accumulatedFilledQty)
				}
				// 价格，数量
				_, quantityGo, err := processSymbolInfo(dataOrder.Symbol, 0, (1-(removeCostRatio/costRatio))*accumulatedFilledQty)
				if err != nil {
					log.Println(err)
					return
				}
				if quantityGo != "" {
					var (
						side         futures.SideType         = "SELL"
						positionSide futures.PositionSideType = "LONG"
					)
					if orderStatus == 3 {
						side = "BUY"
						positionSide = "SHORT"
					}
					_, err := client.NewCreateOrderService().Symbol(dataOrder.Symbol).Type("MARKET").Side(side).PositionSide(positionSide).Quantity(quantityGo).Do(context.Background())
					if err != nil {
						log.Println(err)
						return
					}
				}
				// 下级配置
				costRatio = config.FloatLoss
			}
			for i, clientBelow := range clients {
				belowCostRatio := costRatio * config.Accounts[i].Proportion
				log.Printf("下级：%d", i)
				// 获取账户余额
				belowFree, err := getBalance(clientBelow)
				if err != nil {
					log.Println(err)
					return
				}
				if belowFree <= 0 {
					log.Println("资金为0")
					return
				}
				_, belowQuantityGo, err := processSymbolInfo(dataOrder.Symbol, 0, belowFree*belowCostRatio/lastFilledPrice)
				// log belowFree*belowCostRatio
				log.Println(belowFree * belowCostRatio)
				if err != nil {
					log.Println(err)
					return
				}
				if config.Debug {
					log.Printf("资金：%f", belowFree)
					log.Printf("比例：%f", belowCostRatio)
					log.Printf("币种：%s", dataOrder.Symbol)
					log.Printf("方向：%s", orderMapintSide[orderStatus])
					log.Printf("数量：%s", belowQuantityGo)
					log.Printf("价格：%f", lastFilledPrice)
				}
				var (
					side         futures.SideType         = "BUY"
					positionSide futures.PositionSideType = "LONG"
				)
				if belowQuantityGo != "" {
					if orderStatus == 2 || orderStatus == 4 {
						// 平掉 dataOrder.Symbol 所有仓位
						if orderStatus == 2 {
							side = "SELL"
							positionSide = "LONG"
						} else {
							side = "BUY"
							positionSide = "SHORT"
						}
						_, err := clientBelow.NewCreateOrderService().Symbol(dataOrder.Symbol).Type("MARKET").Side(side).PositionSide(positionSide).Quantity(belowQuantityGo).Do(context.Background())
						if err != nil {
							log.Println(err)
							return
						}
					} else {
						if orderStatus == 1 {
							side = "BUY"
							positionSide = "LONG"
						} else {
							side = "SELL"
							positionSide = "SHORT"
						}
						_, err := clientBelow.NewCreateOrderService().Symbol(dataOrder.Symbol).Type("MARKET").Side(side).PositionSide(positionSide).Quantity(belowQuantityGo).Do(context.Background())
						if err != nil {
							log.Println(err)
							return
						}
					}
				}
			}
		}
	}
}

// 取账余额
func getBalance(c *futures.Client) (free float64, err error) {
	balance, err := c.NewGetBalanceService().Do(context.Background())
	if err != nil {
		return 0, err
	}
	for _, b := range balance {
		if b.Asset == "USDT" {
			free, err = strconv.ParseFloat(b.Balance, 64)
			if err != nil {
				return 0, err
			}
			return free, nil
		}
	}
	return 0, errors.New("no USDT balance")
}

// 处理币种数量和价格
func processSymbolInfo(symbol string, p float64, q float64) (price string, quantity string, err error) {
	var symbolInfo *futures.Symbol
	for _, s := range symbolsInfo {
		if s.Symbol == symbol {
			symbolInfo = &s
		}
	}
	if symbolInfo == nil {
		return "", "", errors.New("symbolInfo is nil")
	}
	if config.Debug {
		log.Println(symbolInfo)
	}
	if q != 0 {
		quantity, err = takeDivisible(q, symbolInfo.Filters[1]["stepSize"].(string))
		if err != nil {
			return "", "", err
		}
	}
	if p != 0 {
		price, err = takeDivisible(p, symbolInfo.Filters[0]["tickSize"].(string))
		if err != nil {
			return "", "", err
		}
	}

	return price, quantity, nil
}

// 调整小数位数并确保可以整除
func takeDivisible(inputVal float64, divisor string) (string, error) {

	divisorVal, err := strconv.ParseFloat(divisor, 64)
	if err != nil || divisorVal == 0 {
		return "", fmt.Errorf("无效的 divisor: %v", err)
	}

	// 计算小数点位数
	decimalPlaces := 0
	if dot := strings.Index(divisor, "."); dot != -1 {
		decimalPlaces = len(divisor) - dot - 1
	}

	// 计算最大整除的值
	quotient := int(inputVal / divisorVal)
	maxDivisible := divisorVal * float64(quotient)

	// 格式化输出
	format := fmt.Sprintf("%%.%df", decimalPlaces)
	return fmt.Sprintf(format, maxDivisible), nil
}

// 风控
func riskGo() {
	for {
		time.Sleep(time.Second * 5)
		// 取 client 仓位信息
		positions, err := client.NewGetPositionRiskService().Do(context.Background())
		if err != nil {
			log.Println(err)
			return
		}
		// for _, position := range positions {

		// }
	}
}
