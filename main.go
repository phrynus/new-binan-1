package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/adshao/go-binance/v2/futures"
)

var (
	client        *futures.Client
	clients       []*futures.Client
	httpClient    *http.Client
	symbolsString []string
	symbols       []futures.Symbol
	listenKey     string
	err           error
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
			symbols = append(symbols, s)
			symbolsString = append(symbolsString, s.BaseAsset)
		}
	}
	if config.Debug {
		log.Println(symbols)
		log.Println(symbolsString)
	}
}

func main() {
	// 启动ws
	go wsUserGo()

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
				false: {"BUY": 1, "SELL": 2},
				true:  {"SELL": 3, "BUY": 4},
			}
			// 1 多下单 2 多平单 3 空下单 4 空平单
			orderStatus := orderMapping[dataOrder.IsReduceOnly][dataOrder.Side]
			cFree, err := getBalance(client)
			if err != nil {
				log.Fatal(err)
			}
			if cFree <= 0 {
				log.Fatal("资金为0")
			}
			// 成交u
			accumulatedFilledQty, err := strconv.ParseFloat(dataOrder.AccumulatedFilledQty, 64)
			lastFilledPrice, err := strconv.ParseFloat(dataOrder.LastFilledPrice, 64)
			if err != nil {
				return
			}
			// 订单 与账户的比例
			costRatio := accumulatedFilledQty * lastFilledPrice / cFree

			if config.Debug {
				log.Printf("当前资金：%f", cFree)
				log.Printf("订单状态：%d", orderStatus)
				log.Printf("成本比例：%f", costRatio)
				log.Printf("订单价格：%f", lastFilledPrice)
				log.Printf("订单数量：%f", accumulatedFilledQty)
			}
			if config.IsFloatLoss && costRatio > config.FloatLoss && (orderStatus == 1 || orderStatus == 3) {
				removeCostRatio := costRatio - config.FloatLoss
				if config.Debug {
					log.Printf("规划订单比例：%f", config.FloatLoss)
					log.Printf("超出规划比例：%f", removeCostRatio)
					log.Printf("剔除数量：%f", (1-(removeCostRatio/costRatio))*accumulatedFilledQty)
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
			// b.Balance 转为 float64
			free, err = strconv.ParseFloat(b.Balance, 64)
			if err != nil {
				return 0, err
			}
			return free, nil
		}
	}
	return 0, errors.New("no USDT balance")
}
