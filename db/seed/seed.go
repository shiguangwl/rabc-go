// Package seed 通过 go:embed 暴露 RBAC 初始数据 JSON。
//
// 数据放在独立的包里而非 internal/server，是因为 go:embed 不允许 ../ 越级访问；
// 把 menu.json 与 embed 声明放在同一目录是 Go 1.16+ 推荐做法。
package seed

import _ "embed"

// MenuJSON 是 RBAC 初始菜单数据，编译期 embed 进二进制。
//
//go:embed menu.json
var MenuJSON string

// ConfigJSON 是系统配置内置项数据，编译期 embed 进二进制。
//
//go:embed config.json
var ConfigJSON string
