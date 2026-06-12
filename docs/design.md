# Nydus 镜像转换流水线触发工具设计文档

## 1. 设计背景

用户已在火山引擎镜像仓库中维护一批待转换镜像，并已在火山引擎持续交付服务中预先创建 Nydus 转换流水线。本工具实现为 Go CLI，用于从本地文件读取一批镜像地址，根据镜像仓库名找到对应流水线，并通过火山引擎 CP `RunPipeline` API 触发流水线执行。

流水线内部负责完成实际 Nydus 转换。本工具不直接执行 `nydusify`、`nydus-image` 等本地转换命令，而是负责批量解析、映射、触发和结果汇总。

当前 MVP 已完成真实火山引擎 CP `RunPipeline` 适配，并已通过真实 AK/SK、真实镜像列表和真实流水线映射完成端到端触发验证。

## 2. 设计目标

### 2.1 核心目标

1. 从本地文件加载一批镜像地址。
2. 逐行解析镜像地址，提取仓库名和镜像 tag。
3. 维护并加载“仓库名 -> 流水线”的映射关系。
4. 根据仓库名找到实际要触发的火山引擎持续交付流水线。
5. 调用火山引擎 CP `RunPipeline` API 顺序触发流水线。
6. 成功触发后记录 `RunPipeline` 返回的执行记录 ID，并在输出结果中体现。
7. 触发时固定传入两个动态变量：
   - `imageToConvert`：当前镜像地址完整内容。
   - `tag`：从当前镜像地址解析出的镜像 tag。
8. 对真实 `RunPipeline` 调用进行限速，保证当前 CLI 进程内 1 分钟最多触发 100 次。
9. 支持 dry-run，用于在不真实触发流水线的情况下验证输入、解析结果和请求摘要。
10. 输出批量执行结果，便于人工查看或机器解析。

### 2.2 非目标

MVP 阶段不做以下事情：

1. 不创建镜像仓库。
2. 不创建或自动发现流水线。
3. 不在本地执行 Nydus 转换。
4. 不实现镜像仓库内容扫描。
5. 不实现复杂任务调度系统。
6. 不等待所有流水线执行完成；当前只负责触发并记录 `RunPipeline` 返回的执行记录 ID。
7. MVP 不支持并发触发 `RunPipeline`；即使未来支持并发，也必须满足全局限速约束。
8. MVP 不实现跨进程、跨机器或分布式限流；限速范围仅限当前 CLI 进程内的一次批量运行。

## 3. 总体架构

```text
┌────────────────────┐
│      CLI 层         │
│ 参数解析/校验       │
└─────────┬──────────┘
          │
          ▼
┌────────────────────┐
│      配置层         │
│ 环境变量/参数合并   │
└─────────┬──────────┘
          │
          ▼
┌────────────────────┐
│   镜像列表加载器    │
│ 读取本地文件        │
└─────────┬──────────┘
          │
          ▼
┌────────────────────┐
│   镜像解析模块      │
│ repo/tag 提取       │
└─────────┬──────────┘
          │
          ▼
┌────────────────────┐
│   映射查询模块      │
│ repo -> pipeline ref│
└─────────┬──────────┘
          │
          ▼
┌────────────────────┐
│ 顺序调度与限速模块  │
│ sequential + 100/min│
└─────────┬──────────┘
          │
          ▼
┌────────────────────┐
│   流水线执行抽象    │
│ PipelineExecutor    │
└─────────┬──────────┘
          │
          ▼
┌────────────────────┐
│ 火山引擎 CP 适配层  │
│ RunPipeline API     │
└────────────────────┘
```

核心原则：

- CLI 层只负责参数解析和命令组织。
- 镜像解析、映射加载、流水线执行、火山引擎 API 调用分别独立封装。
- 业务层只感知“运行某个流水线并传入动态变量”，不直接依赖 `RunPipeline` 的底层请求字段。
- 火山引擎 CP SDK 调用字段集中在火山引擎 CP 适配层处理。
- 批量触发必须由统一调度模块控制，禁止绕过调度模块并发调用 `RunPipeline`。
- 真实 `RunPipeline` 调用必须经过限速器，保证当前进程内 1 分钟最多 100 次。
- dry-run 不调用 `RunPipeline`，因此不消耗限速额度。

## 4. 批量处理流程

```text
读取 images-file
  -> 跳过空行和注释行
  -> 对每行镜像地址执行：
      1. 解析镜像地址
      2. 提取 repository 和 tag
      3. 使用 repository 查询 mapping-file
      4. 构造动态变量：
           imageToConvert = 原始镜像地址
           tag = 解析出的 tag
      5. dry-run：输出请求摘要，不调用 API
         非 dry-run：等待限速许可后顺序调用 RunPipeline
      6. 从成功响应中提取执行记录 ID
      7. 记录单条结果
  -> 汇总成功/失败/跳过数量
  -> 按指定格式输出结果
```

批量处理必须保持输入顺序。第 N 行镜像完成解析、映射、限速等待、`RunPipeline` 调用和结果记录后，才允许处理第 N+1 行镜像。

真实触发模式下，`RunPipeline` 调用必须满足：

1. 严格顺序触发，不并发调用。
2. 当前 CLI 进程内，任意 1 分钟最多发起 100 次 `RunPipeline` 请求。
3. 推荐采用固定间隔限速：两次 `RunPipeline` 请求开始时间之间至少间隔 `time.Minute / runPipelineQPM`。
4. 默认 `runPipelineQPM=100` 时，两次请求开始时间至少间隔 600ms。
5. 如果一次 `RunPipeline` 调用耗时超过限速间隔，则下一次调用无需额外等待。
6. retry 产生的额外 `RunPipeline` 请求也必须计入限速。
7. dry-run 不调用 `RunPipeline`，不受限速约束。

示例镜像：

```text
vfaas-cn-beijing.cr.volces.com/swe/repo1:h2database__h2database-2346-nydus
```

解析结果：

```text
registry   = vfaas-cn-beijing.cr.volces.com
namespace  = swe
repository = repo1
tag        = h2database__h2database-2346-nydus
```

触发流水线时的动态变量：

```text
imageToConvert = vfaas-cn-beijing.cr.volces.com/swe/repo1:h2database__h2database-2346-nydus
tag            = h2database__h2database-2346-nydus
```

## 5. CLI 命令设计

### 5.1 主命令

```bash
nydus-convert run \
  --images-file images.txt \
  --mapping-file pipelines.yaml
```

### 5.2 参数设计

| 参数 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--images-file` | 是 | 无 | 本地镜像列表文件路径。 |
| `--mapping-file` | 是 | 无 | 仓库名到流水线的映射文件路径。 |
| `--region` | 否 | 环境变量或默认配置 | 火山引擎地域。 |
| `--ak` | 否 | 环境变量 | 火山引擎 Access Key；支持命令行传入，但更建议使用环境变量。 |
| `--sk` | 否 | 环境变量 | 火山引擎 Secret Key；支持命令行传入，但更建议使用环境变量。 |
| `--dry-run` | 否 | `false` | 只解析输入并输出请求摘要，不触发流水线。 |
| `--output` | 否 | `text` | 输出格式，支持 `text`、`json`。 |
| `--concurrency` | 否 | `1` | MVP 固定为 `1`，表示顺序触发；传入非 `1` 值应返回配置错误。 |
| `--run-pipeline-qpm` | 否 | `100` | 当前进程内每分钟最多触发 `RunPipeline` 的次数，取值范围 `1-100`。 |
| `--continue-on-error` | 否 | `true` | 单条失败后是否继续处理后续镜像。 |
| `--timeout` | 否 | 待定 | 单次 API 调用超时时间。 |

`--run-pipeline-qpm` 只限制真实 `RunPipeline` 请求；dry-run 不消耗额度。若用户传入大于 100 或小于 1 的值，应返回 `INVALID_CONFIG`。

### 5.3 配置优先级

配置来源优先级：

```text
CLI 参数 > 环境变量 > 默认值
```

建议环境变量：

```text
VOLCENGINE_ACCESS_KEY_ID
VOLCENGINE_SECRET_ACCESS_KEY
VOLCENGINE_REGION
```

## 6. 输入文件设计

### 6.1 镜像列表文件

镜像列表文件每行一个镜像地址：

```text
# images.txt
vfaas-cn-beijing.cr.volces.com/swe/repo1:h2database__h2database-2346-nydus
vfaas-cn-beijing.cr.volces.com/swe/repo2:redis__redis-1234-nydus
```

处理规则：

1. 去除每行首尾空白字符。
2. 空行跳过。
3. 以 `#` 开头的注释行跳过。
4. 非空非注释行必须是合法镜像地址。
5. MVP 要求镜像地址必须包含 tag。

### 6.2 仓库流水线映射文件

建议 MVP 优先支持 YAML，后续可扩展 JSON。

`RunPipeline` 的基础入参需要 `workspaceId` 和 `pipelineId`，因此映射文件不能只保存流水线名称，必须保存每个仓库对应流水线的工作区 ID 和流水线 ID。

```yaml
# pipelines.yaml
repositories:
  repo1:
    workspaceId: w-xxx
    pipelineId: p-xxx
    pipelineName: repo1-nydus-convert
  repo2:
    workspaceId: w-yyy
    pipelineId: p-yyy
    pipelineName: repo2-nydus-convert
```

字段说明：

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `workspaceId` | 是 | 火山引擎持续交付工作区 ID，是调用 `RunPipeline` 的基础字段。 |
| `pipelineId` | 是 | 火山引擎持续交付流水线 ID，是调用 `RunPipeline` 的基础字段。 |
| `pipelineName` | 否 | 流水线名称，仅用于人工识别、日志和输出，不作为触发 API 的基础字段。 |
| `description` | 否 | 映射说明。 |

不建议在 MVP 中使用仅包含流水线名称的简化结构，例如：

```yaml
repo1: repo1-nydus-convert
repo2: repo2-nydus-convert
```

原因是流水线名称不足以构造 `RunPipeline` 请求，且可能存在重名或跨 workspace 的歧义。

推荐结构也便于后续扩展描述、启停状态、重试策略等字段：

```yaml
repositories:
  repo1:
    workspaceId: w-xxx
    pipelineId: p-xxx
    pipelineName: repo1-nydus-convert
    description: swe repo1 nydus conversion pipeline
```

## 7. 镜像地址解析设计

### 7.1 解析目标

将镜像地址解析为结构化数据：

```go
type ImageRef struct {
    Raw        string
    Registry   string
    Namespace  string
    Repository string
    Tag        string
    Digest     string
}
```

### 7.2 repository 提取规则

对于以下镜像：

```text
vfaas-cn-beijing.cr.volces.com/swe/repo1:h2database__h2database-2346-nydus
```

路径部分为：

```text
swe/repo1
```

仓库名取路径最后一段：

```text
repo1
```

对于多级路径：

```text
registry.example.com/team/group/repo3:v1
```

默认仓库名取最后一段：

```text
repo3
```

如果未来存在同名仓库冲突，可扩展映射 key 为完整 repository path，例如 `team/group/repo3`。

### 7.3 tag 提取规则

MVP 要求镜像必须包含 tag：

```text
registry/namespace/repo:tag
```

解析时需要注意 registry 可能包含端口：

```text
registry.example.com:5000/ns/repo:v1
```

因此不能简单按第一个 `:` 拆分，应基于最后一个路径段中的 `:` 判断 tag。

### 7.4 digest 处理

MVP 不以 digest 作为主要输入格式。若出现：

```text
registry/ns/repo@sha256:xxxx
```

应返回明确错误：缺少 tag，无法构造流水线动态变量 `tag`。

若出现同时包含 tag 和 digest：

```text
registry/ns/repo:v1@sha256:xxxx
```

可解析出 `tag = v1`，并保留 digest 信息，但实际是否支持需在实现阶段确认。

## 8. 流水线选择设计

流水线选择流程：

```text
ImageRef.Repository
  -> mapping[repository]
  -> PipelineRef
```

示例：

```yaml
repositories:
  repo1:
    workspaceId: w-xxx
    pipelineId: p-xxx
    pipelineName: repo1-nydus-convert
```

输入镜像解析得到：

```text
repository = repo1
```

最终触发流水线：

```text
workspaceId  = w-xxx
pipelineId   = p-xxx
pipelineName = repo1-nydus-convert
```

映射缺失处理：

1. 当前镜像标记为失败。
2. 错误码建议为 `MAPPING_NOT_FOUND`。
3. 若 `--continue-on-error=true`，继续处理后续镜像。
4. 若 `--continue-on-error=false`，立即停止批量执行。

映射项字段校验：

1. `workspaceId` 为空时，当前映射无效。
2. `pipelineId` 为空时，当前映射无效。
3. 映射项字段缺失时错误码建议为 `INVALID_PIPELINE_MAPPING`。
4. `pipelineName` 仅用于展示，缺失不影响触发。

## 9. 流水线执行抽象与火山引擎适配

### 9.1 已实现接口

通过接口抽象流水线执行能力：

```go
type PipelineExecutor interface {
    Run(ctx context.Context, req RunRequest) (*RunResult, error)
}
```

请求结构：

```go
type RunRequest struct {
    Image          ImageRef
    Pipeline       PipelineRef
    DynamicVars    map[string]string
    DryRun         bool
    IdempotencyKey string
}
```

流水线引用结构：

```go
type PipelineRef struct {
    Repository   string
    WorkspaceID  string
    PipelineID   string
    PipelineName string
}
```

其中 `WorkspaceID` 和 `PipelineID` 来自映射文件，是构造 `RunPipeline` 请求的基础字段；`PipelineName` 仅用于日志、dry-run 和结果输出。

结果结构：

```go
type RunResult struct {
    Image             string
    Repository        string
    Tag               string
    WorkspaceID       string
    PipelineID        string
    PipelineName      string
    ExecutionRecordID string
    Status            string
    TriggeredAt       time.Time
    RateLimitWaitMS   int64
    RawResponse       []byte
}
```

`ExecutionRecordID` 表示 `RunPipeline` 成功响应中返回的执行记录 ID。对外 JSON 输出字段继续使用 `run_id`，其值来自 `ExecutionRecordID`。如果 `RunPipeline` 返回成功但响应中无法提取执行记录 ID，应视为异常响应，当前条目标记失败。

### 9.2 固定动态变量

每次触发流水线时必须传入：

```go
DynamicVars: map[string]string{
    "imageToConvert": image.Raw,
    "tag":            image.Tag,
}
```

其中：

- `imageToConvert` 保持输入文件中该行镜像地址的完整内容。
- `tag` 来自镜像地址解析结果。

### 9.3 火山引擎 CP 适配层

`RunPipeline` API 的官方文档地址：

```text
https://api.volcengine.com/api-docs/view?serviceCode=cp&version=2023-05-01&action=RunPipeline
```

当前适配层基于 `volcengine-go-sdk` 调用 CP `RunPipeline`，业务层不直接依赖 SDK 请求结构。火山引擎适配层负责：

1. 根据 `PipelineRef.WorkspaceID` 和 `PipelineRef.PipelineID` 构造 API 请求。
2. 将 `DynamicVars` 转换为 `RunPipeline` 所需的动态变量字段。
3. 使用 AK/SK 完成请求签名。
4. 处理 API 响应，提取执行记录 ID，并转换为 `RunResult.ExecutionRecordID`。
5. 将底层错误转换为稳定的业务错误类型。

已验证的 SDK 字段映射：

```go
(&cp.RunPipelineInput{}).
    SetWorkspaceId(req.Pipeline.WorkspaceID).
    SetId(req.Pipeline.PipelineID).
    SetParameters(params)
```

其中 `params` 由 `RunRequest.DynamicVars` 转换为 `[]*cp.ParameterForRunPipelineInput`，每个变量通过 `SetKey` 和 `SetValue` 传入。

响应处理规则：

1. `RunPipelineWithContext` 返回错误时，转换为 `RUN_PIPELINE_FAILED`。
2. 响应为空、`out.Id` 为空或 `*out.Id == ""` 时，转换为 `RUN_PIPELINE_RESPONSE_INVALID`。
3. 成功时使用 `out.Id` 作为 `ExecutionRecordID`，对外输出为 `run_id`。

真实端到端验证结果：

| 镜像 tag | pipelineName | 执行记录 ID |
| --- | --- | --- |
| `0.0.1` | `nydus-convert-1` | `4ec0ca5dc5714b3c941d33746be2cf28` |
| `0.0.2` | `nydus-convert-1` | `2998ccace24248109535b390c413dc01` |

说明：上述验证使用真实镜像列表、真实 `workspaceId` / `pipelineId` 映射和真实 AK/SK 完成，证明当前字段映射、动态变量传参和执行记录 ID 提取均可用。

## 10. 批量执行策略

### 10.1 默认执行方式

MVP 必须顺序执行：

```text
--concurrency=1
```

处理第 N 条镜像时，必须完成该条的解析、映射、限速等待、`RunPipeline` 调用和结果记录后，才能进入第 N+1 条镜像。

原因：

1. 降低 API 限流风险。
2. 便于观察和排查失败。
3. 便于保持触发顺序、限速行为和执行记录输出稳定可预期。

### 10.2 RunPipeline 限速策略

真实触发模式下必须限制 `RunPipeline` 调用频率：

```text
--run-pipeline-qpm=100
```

规则：

1. 默认值：100。
2. 最大允许值：100。
3. 最小允许值：1。
4. 派生间隔：`time.Minute / runPipelineQPM`。
5. 当 `runPipelineQPM=100` 时，两次 `RunPipeline` 请求开始时间至少间隔 600ms。
6. dry-run 不调用 `RunPipeline`，不消耗限速额度。
7. retry 如后续启用，每一次重试请求都必须重新经过限速器。

限速器接口建议：

```go
type RateLimiter interface {
    Wait(ctx context.Context) (waited time.Duration, err error)
}
```

批量调度伪代码：

```go
for _, item := range items {
    parsed, err := parseImage(item)
    if err != nil {
        recordFailure(item, err)
        if !continueOnError { break }
        continue
    }

    pipeline, err := mapping.Lookup(parsed.Repository)
    if err != nil {
        recordFailure(item, err)
        if !continueOnError { break }
        continue
    }

    if dryRun {
        recordDryRun(item, parsed, pipeline)
        continue
    }

    waited, err := limiter.Wait(ctx)
    if err != nil {
        recordFailure(item, err)
        break
    }

    result, err := executor.Run(ctx, RunRequest{
        Image:       parsed,
        Pipeline:    pipeline,
        DynamicVars: fixedDynamicVars(parsed),
    })
    if err != nil {
        recordFailure(item, err)
        if !continueOnError { break }
        continue
    }

    recordSuccess(item, result.ExecutionRecordID, waited)
}
```

### 10.3 后续并发扩展限制

MVP 不支持并发触发。后续如果引入并发，也必须满足：

1. `RunPipeline` 调用仍由全局限速器统一发放许可。
2. 任意 1 分钟内 `RunPipeline` 调用次数不得超过 100。
3. 输出结果仍按输入顺序排序。
4. 并发只可用于本地解析、映射校验等非 API 调用阶段，不应绕过限速器直接调用 `RunPipeline`。

非 MVP 并发命令示例：

```bash
nydus-convert run \
  --images-file images.txt \
  --mapping-file pipelines.yaml \
  --concurrency 5
```

### 10.4 失败策略

默认建议：

```text
--continue-on-error=true
```

即单个镜像失败不影响后续镜像处理。失败信息进入结果汇总。

若设置：

```text
--continue-on-error=false
```

则首个失败出现后立即停止。

## 11. 输出设计

### 11.1 text 输出

适合人工查看：

```text
[OK]   line=1 repo=repo1 tag=h2database__h2database-2346-nydus workspace=w-xxx pipeline=p-xxx name=repo1-nydus-convert run_id=exec-xxx rate_limit_wait_ms=0
[OK]   line=2 repo=repo2 tag=redis__redis-1234-nydus workspace=w-yyy pipeline=p-yyy name=repo2-nydus-convert run_id=exec-yyy rate_limit_wait_ms=600
[FAIL] line=3 repo=repo3 error=MAPPING_NOT_FOUND message="pipeline mapping not found"

Summary: total=3 success=2 failed=1 skipped=0 run_pipeline_calls=2 run_pipeline_qpm=100
```

`run_id` 为 `RunPipeline` 返回的执行记录 ID。`rate_limit_wait_ms` 为本次调用前因限速等待的时间，仅真实触发时输出。

### 11.2 JSON 输出

适合机器解析：

```json
{
  "summary": {
    "total": 2,
    "success": 2,
    "failed": 0,
    "skipped": 0,
    "run_pipeline_calls": 2,
    "run_pipeline_qpm": 100
  },
  "items": [
    {
      "line": 1,
      "imageToConvert": "vfaas-cn-beijing.cr.volces.com/swe/repo1:h2database__h2database-2346-nydus",
      "repository": "repo1",
      "tag": "h2database__h2database-2346-nydus",
      "workspaceId": "w-xxx",
      "pipelineId": "p-xxx",
      "pipelineName": "repo1-nydus-convert",
      "status": "triggered",
      "run_id": "exec-xxx",
      "rate_limit_wait_ms": 0
    },
    {
      "line": 2,
      "imageToConvert": "vfaas-cn-beijing.cr.volces.com/swe/repo2:redis__redis-1234-nydus",
      "repository": "repo2",
      "tag": "redis__redis-1234-nydus",
      "workspaceId": "w-yyy",
      "pipelineId": "p-yyy",
      "pipelineName": "repo2-nydus-convert",
      "status": "triggered",
      "run_id": "exec-yyy",
      "rate_limit_wait_ms": 600
    }
  ]
}
```

### 11.3 dry-run 输出

dry-run 不触发流水线，只输出解析结果和将要触发的请求摘要：

```json
{
  "dry_run": true,
  "summary": {
    "total": 1,
    "success": 1,
    "failed": 0,
    "skipped": 0,
    "run_pipeline_calls": 0,
    "run_pipeline_qpm": 100
  },
  "items": [
    {
      "line": 1,
      "imageToConvert": "vfaas-cn-beijing.cr.volces.com/swe/repo1:h2database__h2database-2346-nydus",
      "repository": "repo1",
      "tag": "h2database__h2database-2346-nydus",
      "workspaceId": "w-xxx",
      "pipelineId": "p-xxx",
      "pipelineName": "repo1-nydus-convert",
      "status": "dry_run",
      "dynamic_vars": {
        "imageToConvert": "vfaas-cn-beijing.cr.volces.com/swe/repo1:h2database__h2database-2346-nydus",
        "tag": "h2database__h2database-2346-nydus"
      }
    }
  ]
}
```

dry-run 模式不会生成 `run_id`，因为没有真实调用 `RunPipeline`。

## 12. 错误处理设计

建议定义稳定错误码，便于测试和机器解析。

| 错误码 | 场景 | 是否可继续 |
| --- | --- | --- |
| `IMAGE_FILE_NOT_FOUND` | 镜像列表文件不存在 | 否 |
| `MAPPING_FILE_NOT_FOUND` | 映射文件不存在 | 否 |
| `INVALID_IMAGE_REF` | 镜像地址格式非法 | 是 |
| `TAG_NOT_FOUND` | 镜像地址未包含 tag | 是 |
| `MAPPING_NOT_FOUND` | 仓库名未找到流水线映射 | 是 |
| `INVALID_PIPELINE_MAPPING` | 映射项缺失 `workspaceId` 或 `pipelineId` | 是 |
| `INVALID_CONFIG` | AK/SK、region 等配置缺失或非法；或 `--concurrency != 1`；或 `--run-pipeline-qpm` 不在 `1-100` 范围内 | 否 |
| `AUTH_FAILED` | 火山引擎鉴权失败 | 视情况 |
| `PIPELINE_NOT_FOUND` | 目标流水线不存在 | 是 |
| `RUN_PIPELINE_FAILED` | RunPipeline API 返回业务错误 | 是 |
| `RUN_PIPELINE_RESPONSE_INVALID` | RunPipeline 返回成功但响应缺少执行记录 ID，或 ID 为空 | 是 |
| `RATE_LIMITED` | API 限流 | 可重试 |
| `RATE_LIMIT_WAIT_CANCELED` | 等待限速许可时 context 被取消或超时 | 否 |
| `NETWORK_TIMEOUT` | 网络超时 | 可重试 |

补充规则：

1. `RunPipeline` API 返回业务失败：记录为 `RUN_PIPELINE_FAILED`。
2. `RunPipeline` API 返回成功但无法提取执行记录 ID：记录为 `RUN_PIPELINE_RESPONSE_INVALID`。
3. 触发成功但输出写入失败：应作为整体命令失败处理，避免丢失执行记录 ID。
4. 限速等待被取消：停止后续触发，避免不确定状态下继续发起 API 请求。
5. 如果 `RunPipeline` 已成功返回执行记录 ID，但本地结果记录阶段发生异常，应尽量将该 ID 写入 stderr 或最终错误信息中，避免执行记录丢失。

## 13. 重试策略

MVP 可先设计但不强制实现复杂重试。

建议后续支持：

1. 对网络超时、限流、5xx 错误进行有限重试。
2. 对鉴权失败、参数错误、映射缺失不重试。
3. retry 也属于 `RunPipeline` 请求，必须消耗限速额度。
4. 如果 `RunPipeline` 请求已经到达服务端但客户端超时，重试可能触发多条流水线执行记录。MVP 如无服务端幂等参数，不建议默认开启重试。
5. 支持参数：

```text
--retry 3
--retry-interval 2s
```

6. 重试需要记录在结果中，便于排查。

## 14. 目录与模块

当前 Go 工程结构：

```text
cmd/nydus-convert/main.go
internal/app/
internal/cli/
internal/errors/
internal/image/
internal/input/
internal/mapping/
internal/output/
internal/pipeline/
internal/ratelimit/
internal/volcengine/
docs/
go.mod
go.sum
Makefile
README.md
```

模块职责：

| 模块 | 职责 |
| --- | --- |
| `cmd/nydus-convert` | 程序入口。 |
| `internal/app` | 批量处理主流程，串联输入读取、镜像解析、映射查询、限速、执行和结果汇总。 |
| `internal/cli` | CLI 命令、参数定义和校验。 |
| `internal/errors` | 稳定错误码和应用错误类型。 |
| `internal/input` | 镜像列表文件读取、空行和注释过滤。 |
| `internal/image` | 镜像地址解析，提取 repository 和 tag。 |
| `internal/mapping` | 加载仓库到流水线的映射。 |
| `internal/pipeline` | 流水线执行抽象、`RunRequest` / `RunResult`。 |
| `internal/ratelimit` | `RunPipeline` 调用限速，保证当前进程内每分钟最多触发 100 次。 |
| `internal/volcengine` | 基于 `volcengine-go-sdk` 的火山引擎 CP `RunPipeline` API 适配。 |
| `internal/output` | text/json 输出。 |

## 15. 可测试性设计

### 15.1 单元测试

重点覆盖：

1. 镜像列表读取：空行、注释行、非法行。
2. 镜像地址解析：
   - 标准镜像地址。
   - registry 带端口。
   - 多级 namespace。
   - tag 缺失。
   - digest 输入。
3. 映射加载：
   - YAML 解析。
   - repository 命中。
   - repository 缺失。
   - `workspaceId` 缺失。
   - `pipelineId` 缺失。
4. 动态变量构造：
   - `imageToConvert` 等于原始镜像地址。
   - `tag` 等于解析出的镜像 tag。
5. dry-run：
   - 不调用真实 executor。
   - 输出请求摘要。
6. `RunPipeline` 响应解析：
   - 成功响应中包含执行记录 ID。
   - 执行记录 ID 字段为空。
   - 成功响应缺少执行记录 ID。
   - 底层字段名变化时在适配层集中处理。
7. 限速器：
   - `qpm=100` 时派生间隔为 600ms。
   - `qpm=60` 时派生间隔为 1s。
   - `qpm=0` 返回配置错误。
   - `qpm=101` 返回配置错误。
   - dry-run 不调用限速器。

### 15.2 集成测试

使用 mock `PipelineExecutor` 验证完整流程：

```text
images.txt + pipelines.yaml
  -> 读取镜像列表
  -> 解析 repo/tag
  -> 查询 mapping
  -> 构造 DynamicVars
  -> 调用 mock executor
  -> 汇总结果
```

补充验证：

1. 输入多条镜像，mock executor 返回多个不同执行记录 ID，输出中每条成功记录都有 `run_id`。
2. executor 调用顺序与输入行顺序一致。
3. `--continue-on-error=true` 时，中间一条失败不影响后续触发。
4. `--continue-on-error=false` 时，首个失败后停止，不再触发后续镜像。
5. 使用 fake clock 验证两次 `RunPipeline` 开始时间至少间隔 600ms。
6. 验证第 101 次调用不会发生在同一个 60 秒窗口内。
7. 验证 `RunPipeline` 调用耗时超过限速间隔时，不额外等待。
8. 验证 context cancel 时限速等待返回 `RATE_LIMIT_WAIT_CANCELED`。

### 15.3 联调测试

待 `RunPipeline` 参数确认后，使用用户提供 AK/SK 和已有流水线验证：

1. dry-run 输出符合预期。
2. 实际触发流水线成功。
3. CLI 输出中的 `run_id` 与火山引擎控制台执行记录一致。
4. 火山引擎控制台可看到流水线运行记录。
5. 流水线收到 `imageToConvert` 和 `tag` 两个动态变量。
6. 连续触发超过 100 条镜像时，确认不会超过每分钟 100 次。
7. 所有触发仍按输入文件顺序发生。
8. Nydus 转换结果符合预期。

## 16. 后续扩展方向

1. 支持批量任务状态轮询和 `--wait`。
2. 支持生成转换报告文件。
3. 支持从火山引擎镜像仓库自动拉取镜像列表。
4. 支持从火山引擎持续交付服务自动查询流水线列表并生成映射。
5. 支持多 region、多账号配置。
6. 支持更复杂的 tag 生成策略。
7. 支持本地 Nydus 工具链或 Kubernetes Job 作为替代执行后端。
