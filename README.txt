# new-binan-1
币安账号风控以及同步订单到其他账户


```
APIKey            string    `json:"api_key"`
SecretKey         string    `json:"secret_key"`
Proxy             string    `json:"proxy"`             // 代理
FloatLoss         float64   `json:"floatLoss"`         // 浮动止损阈值 止损 基于账户余额
IsFloatLoss       bool      `json:"isFloatLoss"`       // 上方开关
PositionBalance   float64   `json:"positionBalance"`   // 超余额阈值 减仓 基于账户余额
IsPositionBalance bool      `json:"isPositionBalance"` // 上方开关
Debug             bool      `json:"debug"`             // 调试log
Accounts          []Account `json:"accounts"`          // 同步账号列表

APIKey     string  `json:"api_key"`
SecretKey  string  `json:"secret_key"`
Switch     bool    `json:"switch"`     // 是否启用
Proportion float64 `json:"proportion"` // 下单比例 百分比
```

```json
{
  "api_key": "your-api-key",
  "secret_key": "your-secret-key",
  "proxy": "",
  "floatLoss": 0.15,
  "isFloatLoss": true,
  "positionBalance": 0.1,
  "isPositionBalance": true,
  "debug": false,
  "accounts": [
	{
	  "api_key": "your-api-key",
	  "secret_key": "your-secret-key",
	  "switch": true,
	  "proportion": 1
	}
  ]
}
```
多单 下单
{ETHUSDT android_YEouAQLRbkMFSdfCU5OX BUY MARKET GTC 0.023 0 2143.89000 0 TRADE FILLED 8389765854323405707 0.023 0.023 2143.89 USDT 0.02465473 1741522804445 5319160861 0 0 false false CONTRACT_PRICE MARKET BOTH false   false 0 EXPIRE_MAKER NONE 0}
多单 关闭
{ETHUSDT android_J37aXnEoTevOJEzK8REH SELL MARKET GTC 0.023 0 2139.31000 0 TRADE FILLED 8389765854316304270 0.023 0.023 2139.31 USDT 0.02460206 1741522005363 5319108334 0 0 false true CONTRACT_PRICE MARKET BOTH false   false -0.10534000 EXPIRE_MAKER NONE 0}

空单 下单
{ETHUSDT android_pNZejHFYQvRUIT22Zbat SELL MARKET GTC 0.023 0 2133.80000 0 TRADE FILLED 8389765854318753117 0.023 0.023 2133.80 USDT 0.02453870 1741522239998 5319127267 0 0 false false CONTRACT_PRICE MARKET BOTH false   false 0 EXPIRE_MAKER NONE 0}
空单 平
{ETHUSDT android_IUbkqLAm4irGiAv4qoeg BUY MARKET GTC 0.023 0 2140.19000 0 TRADE FILLED 8389765854321324895 0.023 0.023 2140.19 USDT 0.02461218 1741522533079 5319148164 0 0 false true CONTRACT_PRICE MARKET BOTH false   false -0.14697000 EXPIRE_MAKER NONE 0}
