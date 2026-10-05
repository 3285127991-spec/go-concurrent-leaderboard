# go-concurrent-leaderboard

一个纯内存、并发安全的排行榜 Go 库，附命令行演示和自动化测试。
无网络服务、无磁盘持久化、无数据库、无第三方依赖，仅使用 Go 标准库。

## 功能

- 创建时指定最大玩家数量，任何时刻玩家数都不会超过容量
- 每个玩家使用唯一非空字符串 ID 和 int64 分数
- 排名按分数从高到低；分数相同时按玩家 ID 字典序升序
- 所有 API 均可被多个 goroutine 安全并发调用
- 查询结果（Top / Around / Snapshot）返回独立副本，调用方修改不影响内部状态

## API

| 方法 | 说明 |
| --- | --- |
| \`New(capacity int) (*Leaderboard, error)\` | 创建排行榜；capacity <= 0 返回 \`ErrInvalidCapacity\` |
| \`SetScore(id string, score int64) error\` | 设置绝对分数，玩家不存在则创建 |
| \`AddScore(id string, delta int64) error\` | 在原分数上累加，玩家不存在则以 delta 创建 |
| \`Remove(id string) bool\` | 删除玩家，返回值明确表示玩家此前是否存在 |
| \`Get(id string) (int64, bool)\` | 查询玩家当前分数 |
| \`Rank(id string) (int, bool)\` | 查询玩家名次（从 1 开始） |
| \`Top(n int) []Entry\` | 返回前 n 名（n 超过玩家数时返回全部） |
| \`Around(id string, before, after int) ([]Entry, bool)\` | 返回该玩家前后若干名，包含玩家自己 |
| \`Snapshot() []Entry\` | 返回全部条目的稳定有序快照 |
| \`Len() int\` / \`Cap() int\` | 当前玩家数 / 最大容量 |

容量规则：满员时只拒绝**新增**玩家（返回 \`ErrFull\`），已有玩家的
\`SetScore\` / \`AddScore\` 更新不受影响。空 ID 返回 \`ErrEmptyID\`。

## 使用示例

\`\`\`go
lb, err := leaderboard.New(100)
if err != nil {
    log.Fatal(err)
}

_ = lb.SetScore("alice", 100)
_ = lb.AddScore("alice", 25) // 125

rank, ok := lb.Rank("alice")   // 1, true
top10 := lb.Top(10)            // 独立副本
around, ok := lb.Around("alice", 2, 2)
\`\`\`

## 运行 CLI 演示

\`\`\`powershell
go run ./cmd/demo
\`\`\`

## 运行测试

可重复运行的完整验证（格式化检查 + 静态检查 + race detector 测试）：

\`\`\`powershell
gofmt -l .
go vet ./...
go test ./... -race -count=1
\`\`\`

测试覆盖：新增、覆盖分数、累加分数、删除（含删除不存在玩家）、容量边界、
并列分数排序、Rank、Top、Around、Snapshot 稳定顺序、结果副本隔离、
非法参数、并发读写，以及并发压力下容量不超限。

## 项目结构

\`\`\`
leaderboard.go        核心库（package leaderboard）
leaderboard_test.go   自动化测试
cmd/demo/main.go      命令行演示
\`\`\`

## 实现说明

内部维护一个 map（ID -> 分数）和一个始终按排名顺序排序的切片，
写入时通过二分查找定位插入/删除位置，查询时直接按序切片返回副本。
读写由 \`sync.RWMutex\` 保护：写操作互斥，读操作可并发。
