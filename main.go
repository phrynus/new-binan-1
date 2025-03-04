package main

import (
	"context"
	"crypto/tls"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
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
	// 监听 OS 信号，优雅退出
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
		wsHandler := func(event *futures.WsUserDataEvent) {
			if config.Debug {
				log.Println(event)
			}
		}
		wsErrHandler := func(err error) {
			if config.Debug {
				log.Println("wsErrorHandler:", err)
			}
			time.Sleep(time.Duration(2<<retryCount) * time.Second)
		}
		doneC, _, err := futures.WsUserDataServe(listenKey, wsHandler, wsErrHandler)
		if err != nil {
			log.Fatal(err)
			if config.Debug {
				log.Println("WsUserGo 重试中...")
			}
			time.Sleep(time.Second * 5) // 初始等待 5s 再重试
			continue
		}
		retryCount = 0
		log.Println("开始监听")
		<-doneC
	}
}
