# SUB2 自定义镜像更新指南

本文档记录我们当前 SUB2 自定义镜像里保留的业务改动，以及以后合并官方新版源码时应该怎么更新。

核心原则：

- 不要直接用官方源码或官方镜像覆盖线上 SUB2。
- 所有官方更新都先合并进我们的自定义分支。
- 合并后重新构建 `gaoge-sub2api:*` 自定义镜像，再部署。
- 线上运行失败时，优先回滚到上一版已验证镜像。

## 1. 当前固定分支

SUB2 自定义分支：

```bash
codex/platform-subscription-billing-v1
```

远程仓库：

```bash
origin   https://github.com/KKK-898/sub2api.git
upstream https://github.com/Wei-Shaw/sub2api.git
```

当前自定义分支基于官方源码，同时保留我们的支付桥接和平台订阅扣费规则。以后更新官方代码时，必须在这个分支上合并官方新版。

线上服务器源码目录：

```bash
/opt/sub2api/source
```

线上容器名：

```bash
sub2api
```

当前镜像命名规则：

```bash
gaoge-sub2api:<自定义功能名>-<时间戳>
```

示例：

```bash
gaoge-sub2api:platform-subscription-eligibility-fix-20260704-083710
```

## 2. 当前自定义功能说明

### 2.1 支付桥接保留

我们之前已经在自定义镜像中保留了支付桥接逻辑，用于配合软件后台的主支付入口、备用支付入口等能力。

以后合并官方代码时，不能删除这部分自定义支付桥接逻辑。

### 2.2 平台订阅扣费规则

这次新增的平台订阅扣费规则用于让软件后台控制 SUB2 的请求前计费判断。

规则只对有平台订阅的用户生效：

- 无平台订阅用户：不走平台订阅规则，继续按 SUB2 原逻辑扣余额。
- 有平台订阅用户：SUB2 读取软件后台规则，决定本次请求走平台订阅、走用户余额或拒绝。
- 未命中规则：默认走平台订阅。
- 平台订阅额度不足：回退检查用户个人余额。
- 明确命中 `balance`：只走用户个人余额，不扣平台订阅额度。
- 明确命中 `deny`：请求前拒绝。

重要：SUB2 物理扣费仍然扣用户 `balance`。平台订阅和用户个人余额的区分依赖“归因字段”和“余额隔离计算”。

余额隔离公式：

```text
平台订阅剩余额度 = 今日 grant 金额 - 今日平台订阅归因已用额度
用户个人可用余额 = SUB2 总余额 - 平台订阅剩余额度
```

### 2.3 旧账单兼容

上线平台订阅归因字段之前产生的 usage log 没有 `platform_subscription_action`。

为了兼容历史当天消费，平台订阅统计接口会把以下账单计入平台订阅消费：

- `platform_subscription_action = 'subscription'`
- `platform_subscription_action IS NULL`
- `platform_subscription_action = ''`

明确标记为 `balance` 的账单不会计入平台订阅消费。

这个兼容逻辑很重要。否则当天已经发生的旧消费会被误认为没有消耗平台订阅额度，导致“月卡额度已实际用完但系统还认为有剩余”，从而影响余额回退判断。

### 2.4 多后台域名容灾

SUB2 拉取软件后台内部接口时支持多个后台地址，当前配置为：

```bash
${ADMIN_PUBLIC_URL}
```

如果第一个域名异常，会继续尝试后面的域名。

软件后台 Nginx 需要代理：

```text
/internal/sub2/platform-subscription/*
```

并限制为服务器本机、内网或可信来源访问。

### 2.5 Redis 缓存和计数

新增 Redis key：

```text
platform_sub:rules
platform_sub:user:{user_id}:today
platform_sub:used:{user_id}:{grant_date}
```

默认缓存时间：

```text
规则缓存 TTL：600 秒
用户订阅状态缓存 TTL：120 秒
HTTP 超时：3 秒
```

清缓存命令：

```bash
docker exec sub2api-redis redis-cli DEL platform_sub:rules

for key in $(docker exec sub2api-redis redis-cli --raw --scan --pattern "platform_sub:user:*:today"); do
  docker exec sub2api-redis redis-cli DEL "$key" >/dev/null
done
```

### 2.6 SUB2 环境变量

线上 `.env` 需要保留：

```bash
PLATFORM_SUBSCRIPTION_BILLING_ENABLED=true
PLATFORM_SUBSCRIPTION_BILLING_BACKEND_URL=${ADMIN_PUBLIC_URL}
PLATFORM_SUBSCRIPTION_BILLING_INTERNAL_TOKEN=<不要写入文档或提交>
PLATFORM_SUBSCRIPTION_BILLING_RULES_TTL_SECONDS=600
PLATFORM_SUBSCRIPTION_BILLING_USER_TTL_SECONDS=120
PLATFORM_SUBSCRIPTION_BILLING_HTTP_TIMEOUT_SECONDS=3

SOFTWARE_ADMIN_ONLINE_PAYMENT_CONFIG_URL=
SOFTWARE_ADMIN_API_BASE_URL=${ADMIN_PUBLIC_URL}/api
PAYMENT_BACKUP_BRIDGE_URL=${PAY_PUBLIC_URL}/pay
PAYMENT_BACKUP_BRIDGE_SECRET=<不要写入文档或提交>
```

注意：

- `INTERNAL_TOKEN` 只能保存在服务器 `.env` 或软件后台配置中。
- `PAYMENT_BACKUP_BRIDGE_SECRET` 只能保存在服务器 `.env` 或软件后台配置中。
- 不要提交 token、数据库密码、JWT 密钥、支付密钥。

## 3. 当前自定义代码文件

### 3.1 SUB2 主要改动文件

平台订阅计费服务：

```text
backend/internal/service/platform_subscription_billing.go
backend/internal/repository/platform_subscription_usage_counter.go
```

计费链路接入：

```text
backend/internal/service/billing_cache_service.go
backend/internal/service/gateway_service.go
backend/internal/service/openai_gateway_service.go
backend/internal/handler/ops_error_logger.go
```

usage log 归因和统计：

```text
backend/internal/service/usage_log.go
backend/internal/repository/usage_log_repo.go
backend/internal/service/usage_service.go
backend/internal/service/account_usage_service.go
backend/internal/handler/admin/usage_handler.go
backend/internal/handler/dto/types.go
backend/internal/handler/dto/mappers.go
```

路由、wire、配置：

```text
backend/internal/config/config.go
backend/internal/server/routes/admin.go
backend/internal/repository/wire.go
backend/internal/service/wire.go
backend/cmd/server/wire_gen.go
```

数据库迁移：

```text
backend/migrations/154_platform_subscription_usage_attribution.sql
```

测试更新：

```text
backend/internal/repository/usage_log_repo_request_type_test.go
```

### 3.2 软件后台配套改动

软件后台仓库：

```text
D:\代码\ruanjianhoutai
```

相关提交：

```bash
c56f39f Add platform subscription billing rules
```

主要文件：

```text
server/platform-subscriptions.js
server/server.js
src/pages/software/PlatformSubscriptionManagePage.tsx
src/services/api.ts
```

软件后台提供：

```text
GET /internal/sub2/platform-subscription/billing-rules
GET /internal/sub2/platform-subscription/users/{user_id}/today
```

软件后台前端提供“平台订阅 > 扣费规则”管理页面。

## 4. 以后更新官方 SUB2 的标准流程

以下流程建议在本地或服务器源码目录执行。不要在有未保存改动的目录里直接开始。

### 4.1 更新前检查

```bash
cd /opt/sub2api/source
git status -sb
git branch --show-current
git log -1 --oneline --decorate
```

必须确认当前分支是：

```bash
codex/platform-subscription-billing-v1
```

如果不是，先切换：

```bash
git switch codex/platform-subscription-billing-v1
```

如果工作区不干净，先判断改动是否需要保留：

```bash
git status --short
git diff --stat
```

不要使用 `git reset --hard` 直接丢改动，除非已经确认这些改动不需要。

### 4.2 拉取官方更新

```bash
git fetch upstream
git fetch origin
```

查看官方最新提交：

```bash
git log --oneline --decorate --max-count=10 upstream/main
```

也可以按官方 tag 更新：

```bash
git tag --sort=-v:refname | head
```

### 4.3 创建更新分支

建议每次官方更新都从自定义分支开一个临时更新分支：

```bash
git switch codex/platform-subscription-billing-v1
git pull --ff-only origin codex/platform-subscription-billing-v1
git switch -c codex/platform-subscription-billing-v1-update-YYYYMMDD
```

示例：

```bash
git switch -c codex/platform-subscription-billing-v1-update-20260713
```

### 4.4 合并官方代码

合并官方 main：

```bash
git merge upstream/main
```

如果是合并某个官方 tag：

```bash
git merge v0.1.xxx
```

出现冲突时，重点检查这些区域：

```text
backend/internal/service/billing_cache_service.go
backend/internal/service/gateway_service.go
backend/internal/service/openai_gateway_service.go
backend/internal/repository/usage_log_repo.go
backend/internal/config/config.go
backend/cmd/server/wire_gen.go
backend/internal/service/wire.go
backend/internal/repository/wire.go
backend/internal/handler/dto/types.go
backend/internal/handler/dto/mappers.go
```

冲突处理原则：

- 保留官方新版的新增能力。
- 保留我们的平台订阅规则判断。
- 保留支付桥接相关自定义逻辑。
- 不要删除 `platform_subscription_*` usage log 字段。
- 不要删除 `GetPlatformSubscriptionUsageStats` 里兼容 NULL/空 action 的统计逻辑。
- 不要删除 `PLATFORM_SUBSCRIPTION_BILLING_*` 配置。
- 不要删除 Redis counter 逻辑。

冲突解决后：

```bash
git status --short
git add <resolved-files>
git commit
```

## 5. 合并后必须检查的逻辑点

### 5.1 请求前检查

确认 `BillingCacheService.CheckBillingEligibility` 中仍然存在平台订阅决策逻辑：

```text
resolvePlatformSubscriptionBillingDecision
checkPlatformSubscriptionEligibility
```

必须满足：

- 有平台订阅决策时，不走 SUB2 官方订阅分组检查。
- `subscription` action 有剩余额度时允许请求。
- `subscription` action 额度不足时回退个人余额。
- `balance` action 只检查个人余额。
- `deny` action 直接拒绝。
- 无平台订阅决策时走原 SUB2 逻辑。

### 5.2 请求后归因

确认 `gateway_service.go` 和 `openai_gateway_service.go` 中仍然会：

- 从 context 读取 `PlatformSubscriptionDecisionFromContext(ctx)`。
- 平台订阅规则命中时，物理扣费仍走用户 balance。
- usage log 写入：

```text
platform_subscription_action
platform_subscription_rule_id
platform_subscription_grant_id
platform_subscription_grant_date
```

- 只有真实落账成功后，才累加 Redis 今日平台订阅已用额度。
- `balance` action 不增加平台订阅已用额度。

### 5.3 平台订阅统计

确认 `usage_log_repo.go` 中平台订阅统计条件包含：

```sql
platform_subscription_action = 'subscription'
OR platform_subscription_action IS NULL
OR platform_subscription_action = ''
```

并且不能包含：

```sql
platform_subscription_action = 'balance'
```

### 5.4 内部接口兼容

确认软件后台内部接口仍然返回：

```json
{
  "rules": [],
  "default_action": "subscription"
}
```

默认必须是：

```text
subscription
```

不要改回 `balance`，否则空规则时订阅用户会被默认扣个人余额。

## 6. 测试流程

在 SUB2 后端目录执行：

```bash
cd backend
go test ./internal/service ./internal/repository ./internal/handler/admin ./internal/handler/dto ./internal/server
```

如果时间允许，再执行全量测试：

```bash
go test ./...
```

必须重点通过：

```bash
go test ./internal/service
go test ./internal/repository
```

## 7. 构建自定义镜像

在 SUB2 根目录执行：

```bash
cd /opt/sub2api/source
STAMP=$(date +%Y%m%d-%H%M%S)
OFFICIAL_VERSION="0.1.152"
IMAGE="gaoge-sub2api:v${OFFICIAL_VERSION}-platform-subscription-$STAMP"

docker build \
  -t "$IMAGE" \
  --build-arg VERSION="$OFFICIAL_VERSION" \
  --build-arg COMMIT="$(git rev-parse --short=12 HEAD)" \
  --build-arg GOPROXY=https://goproxy.cn,direct \
  --build-arg GOSUMDB=sum.golang.google.cn \
  -f Dockerfile .
```

注意：

- `VERSION` 必须使用纯官方版本号，例如 `0.1.152`。
- 不要写成 `v0.1.152`，否则前端会显示成 `vv0.1.152`。
- 不要写成 `0.1.152-gaoge` 或 `v0.1.152-gaoge`。
- 自定义标记放在 Docker 镜像名里，例如 `gaoge-sub2api:v0.1.152-platform-subscription-20260713-091535`。
- 自定义构建必须使用 `BuildType=custom`，后台官方在线更新和回滚接口必须返回 `SELF_UPDATE_DISABLED`。
- 管理后台不得展示官方镜像或官方回滚命令；自定义版本只能通过中转仓库、主仓库和自定义镜像发布流程更新。

构建成功后再切换容器。不要在构建失败时停止旧容器。

## 8. 部署自定义镜像

推荐保留旧容器作为回滚点：

```bash
STAMP=$(date +%Y%m%d-%H%M%S)
IMAGE="gaoge-sub2api:platform-subscription-$STAMP"
BACKUP_CONTAINER="sub2api-before-platform-subscription-$STAMP"
ENV_FILE="/opt/sub2api/.env"

docker stop sub2api
docker rename sub2api "$BACKUP_CONTAINER"

docker run -d \
  --name sub2api \
  --restart unless-stopped \
  --network sub2api_sub2api-network \
  -p 127.0.0.1:8080:8080 \
  --env-file "$ENV_FILE" \
  -v /opt/sub2api/data:/app/data \
  "$IMAGE"
```

如果新容器启动失败，回滚：

```bash
docker rm -f sub2api || true
docker rename "$BACKUP_CONTAINER" sub2api
docker start sub2api
```

## 9. 部署后验证

### 9.1 容器健康

```bash
docker ps --format 'table {{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}'
docker inspect -f '{{.State.Health.Status}}' sub2api
docker logs --tail 120 sub2api
```

期望：

```text
sub2api healthy
```

### 9.2 SUB2 管理统计接口

未带管理员认证时，返回 `401/403` 是正常的，说明路由存在：

```bash
curl -sS -m 5 -o /tmp/sub2api_platform_stats.out -w '%{http_code}' \
  http://127.0.0.1:8080/api/v1/admin/usage/platform-subscription
```

期望：

```text
401 或 403
```

如果带管理员 token，应返回统计 JSON。

### 9.3 软件后台内部接口

不要在命令输出中打印 token。可以从配置读取 token 后请求：

```bash
TOKEN=$(python3 -c "import json; from pathlib import Path; c=json.loads(Path('/www/ruanjianhoutai/shared/data/sms-receiver-config.json').read_text()); print(c.get('platformSubscriptionInternalToken') or c.get('platform_subscription_internal_token') or c.get('sub2AdminToken') or '')")

curl -sS -m 8 \
  -H "Authorization: Bearer $TOKEN" \
  http://127.0.0.1:8788/internal/sub2/platform-subscription/billing-rules
```

期望返回：

```json
{
  "success": true,
  "data": {
    "rules": [],
    "default_action": "subscription"
  }
}
```

### 9.4 容器内访问后台域名

```bash
docker exec -e TOKEN="$TOKEN" sub2api sh -lc '
for url in \
  "${ADMIN_PUBLIC_URL}"
do
  echo "$url"
  wget -q -T 8 --header="Authorization: Bearer $TOKEN" \
    -O - "$url/internal/sub2/platform-subscription/billing-rules" | head -c 300
  echo
done
'
```

期望三个域名都返回 JSON，不是 HTML。

如果返回 HTML，通常是 Nginx 没有代理：

```text
/internal/sub2/
```

### 9.5 指定用户验证

示例用户：

```text
user_id = 1506
email = 15607365013@163.COM
```

查询用户订阅状态：

```bash
curl -sS -m 8 \
  -H "Authorization: Bearer $TOKEN" \
  http://127.0.0.1:8788/internal/sub2/platform-subscription/users/1506/today
```

重点看：

```json
{
  "has_subscription": true,
  "grant_amount_usd": 50,
  "used_usd": 47.760368,
  "remaining_usd": 2.239632
}
```

如果 `used_usd` 明显偏低，检查 SUB2 平台订阅统计是否包含旧空归因账单。

## 10. 规则配置建议

软件后台路径：

```text
服务管理 -> 平台订阅 -> 扣费规则
```

推荐规则：

```text
Codex 分组 -> subscription
Claude 专属分组 -> subscription
Claude 普通分组 -> balance
明确禁止的模型/分组 -> deny
```

建议优先按分组 ID 配置，比按模型名更稳定。

示例：

```text
match_type: group
match_mode: exact
match_value: 31
action: subscription
priority: 10
```

默认未命中：

```text
subscription
```

所以不配置规则时，订阅用户保持旧逻辑：优先走平台订阅，额度不够再走余额。

## 11. 常见问题

### 11.1 用户有余额但提示余额不足

优先检查：

```bash
docker logs --since 10m sub2api 2>&1 | grep -E 'billing_eligibility_check_failed|platform subscription eligibility failed|balance eligibility failed'
```

如果看到：

```text
platform subscription eligibility failed
```

重点看日志里的：

```text
action
total_balance
remaining
personal
reason
```

如果软件后台显示月卡剩余很多，但 SUB2 总余额很少，通常是平台订阅已用统计漏算旧账单。

### 11.2 配置规则后没立即生效

清规则缓存：

```bash
docker exec sub2api-redis redis-cli DEL platform_sub:rules
```

清用户订阅状态缓存：

```bash
for key in $(docker exec sub2api-redis redis-cli --raw --scan --pattern "platform_sub:user:*:today"); do
  docker exec sub2api-redis redis-cli DEL "$key" >/dev/null
done
```

### 11.3 软件后台接口失败

SUB2 的策略是 fail-open：

- 有缓存则使用缓存。
- 冷启动无缓存时，降级到现有 SUB2 逻辑，优先保证用户可用。

检查：

```bash
docker logs --since 10m sub2api 2>&1 | grep -i 'platform subscription'
```

### 11.4 下次官方更新覆盖了我们的逻辑

恢复方式：

```bash
git switch codex/platform-subscription-billing-v1
git log --oneline --decorate --max-count=5
```

确认最新自定义提交仍在。如果不在，需要从 GitHub 拉回：

```bash
git fetch origin
git switch codex/platform-subscription-billing-v1
git reset --hard origin/codex/platform-subscription-billing-v1
```

注意：`reset --hard` 会丢弃当前工作区改动，只能在确认不需要本地改动时使用。

## 12. 推荐更新 checklist

每次更新官方 SUB2 前后按这个清单确认：

- 当前分支是 `codex/platform-subscription-billing-v1`。
- 工作区无未确认改动。
- 已创建本次更新临时分支。
- 已 `git fetch upstream`。
- 已合并官方 `upstream/main` 或指定 tag。
- 如果不是直接从官方 tag 构建，已同步 `backend/cmd/server/VERSION` 到当前官方版本号，避免后台误报“有新版本可用”。
- Docker 和 GoReleaser 构建均使用 `BuildType=custom`，更新与回滚 API 返回 `SELF_UPDATE_DISABLED`。
- 冲突处理后保留支付桥接逻辑。
- 冲突处理后保留平台订阅扣费规则。
- `default_action` 仍然是 `subscription`。
- usage log 归因字段仍然存在。
- 平台订阅统计仍然兼容 NULL/空 action。
- Go 测试通过。
- 新 Docker 镜像构建成功。
- 新容器 healthy。
- 后台三个域名内部接口返回 JSON。
- 规则缓存已清理。
- 旧容器已保留，便于回滚。
- 新镜像验证稳定后，再考虑清理旧镜像。

## 13. 一句话流程

```text
官方新版 -> 合并到 codex/platform-subscription-billing-v1 -> 解决冲突 -> 跑测试 -> 构建 gaoge-sub2api 自定义镜像 -> 部署 -> 验证 -> 推送分支
```

不要直接：

```text
官方源码覆盖 /opt/sub2api/source
```

也不要直接：

```text
docker run 官方原版镜像
```
