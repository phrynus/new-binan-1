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
```

```json
{
  "api_key": "your-api-key",
  "secret_key": "your-secret-key",
  "proxy": "",
  "floatLoss": 15,
  "isFloatLoss": true,
  "positionBalance": 10,
  "isPositionBalance": true,
  "debug": true,
  "accounts": [
    {
      "api_key": "your-api-key",
      "secret_key": "your-secret-key",
      "switch": true
    },
    {
      "api_key": "your-api-key",
      "secret_key": "your-secret-key",
      "switch": true
    }
  ]
}
```