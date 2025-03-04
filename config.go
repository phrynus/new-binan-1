package main

import (
	"encoding/json"
	"log"
	"os"
)

var config Config

type Account struct {
	APIKey    string `json:"api_key"`
	SecretKey string `json:"secret_key"`
	Switch    bool   `json:"switch"` // 是否启用
}

type Config struct {
	APIKey            string    `json:"api_key"`
	SecretKey         string    `json:"secret_key"`
	Proxy             string    `json:"proxy"`             // 代理
	FloatLoss         float64   `json:"floatLoss"`         // 每次下单的浮亏
	IsFloatLoss       bool      `json:"isFloatLoss"`       // 是否使用浮亏
	PositionBalance   float64   `json:"positionBalance"`   // 每次下单的仓位
	IsPositionBalance bool      `json:"isPositionBalance"` // 是否使用仓位
	Debug             bool      `json:"debug"`             // 是否调试
	Accounts          []Account `json:"accounts"`          // 附加账号列表
}

func init() {
	b, err := os.ReadFile("config.json")
	if err != nil {
		log.Fatal(err)
	}
	err = json.Unmarshal(b, &config)
	if err != nil {
		log.Fatal(err)
	}
}
