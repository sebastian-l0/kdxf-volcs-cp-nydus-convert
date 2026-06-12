# kdxf-volcs-cp-nydus-convert

通过火山引擎持续交付流水线，批量触发已有镜像的 Nydus 转换。

## 项目背景

用户已在火山引擎镜像仓库中维护一批待转换镜像，并已在火山引擎持续交付服务中预先创建 Nydus 转换流水线。本项目规划提供一个 Go CLI 工具，从本地文件加载镜像地址列表，解析镜像仓库名与 tag，通过仓库名映射到实际需要触发的流水线，并调用火山引擎 CP `RunPipeline` API 运行流水线。

流水线固定使用两个动态变量：

- `imageToConvert`：镜像地址文件中的某一行完整内容。
- `tag`：从 `imageToConvert` 中解析出的镜像 tag。

## 当前状态

Go CLI MVP 已实现，并已通过真实火山引擎 CP `RunPipeline` 端到端触发测试。

当前支持：

- 从本地文件读取镜像列表，自动跳过空行和 `#` 注释行。
- 解析镜像地址中的 `registry`、`namespace`、`repository`、`tag`。
- 从 YAML 映射文件加载 `repository -> workspaceId + pipelineId`。
- dry-run 输出解析结果和固定动态变量摘要。
- 使用 `volcengine-go-sdk` 真实调用 CP `RunPipeline`。
- 顺序触发流水线，并通过 `--run-pipeline-qpm` 控制当前进程内每分钟调用次数，最大 100。
- 输出 text/json 两种结果格式，并记录成功触发后返回的执行记录 ID（`run_id`）。

## 快速开始

### 构建

```bash
make build
```

构建产物：

```text
build/nydus-convert
```

### 准备镜像列表

```text
# images.txt
cp-enterprise-cn-beijing.cr.volces.com/hxy-test/golang-demo-app:0.0.1
cp-enterprise-cn-beijing.cr.volces.com/hxy-test/golang-demo-app:0.0.2
```

### 准备流水线映射

```yaml
# pipelines.yaml
repositories:
  golang-demo-app:
    workspaceId: 54308da883254c76b658500c3b75da77
    pipelineId: 06141abe4e974a8e932d9f5f408dcde8
    pipelineName: nydus-convert-1
```

### dry-run 验证

```bash
./build/nydus-convert run \
  --images-file ./tmp/images.txt \
  --mapping-file ./tmp/pipelines.yaml \
  --dry-run \
  --output json
```

### 真实触发 RunPipeline

建议通过环境变量提供 AK/SK：

```bash
export VOLCENGINE_ACCESS_KEY_ID='your-ak'
export VOLCENGINE_SECRET_ACCESS_KEY='your-sk'
export VOLCENGINE_REGION='cn-beijing'

./build/nydus-convert run \
  --images-file ./tmp/images.txt \
  --mapping-file ./tmp/pipelines.yaml \
  --output json
```

也可以通过 CLI 参数传入：

```bash
./build/nydus-convert run \
  --images-file ./tmp/images.txt \
  --mapping-file ./tmp/pipelines.yaml \
  --region cn-beijing \
  --ak '<your-ak>' \
  --sk '<your-sk>' \
  --output json
```

## 常用 Makefile 命令

```bash
make fmt    # gofmt -w cmd internal
make test   # go test ./...
make build  # go build -o build/nydus-convert ./cmd/nydus-convert
make all    # fmt + test + build
make clean  # 清理构建产物
```

## 文档

- [设计文档](docs/design.md)
