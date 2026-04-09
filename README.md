项目结构：
```
llm-platform/
├── cmd/                              # 可执行应用入口
│   └── api/                          # REST + gRPC API 网关
│       └── main.go                   
│   └── worker                        # 后台Worker（文档处理、定时任务）
│       └── main.go                   # 文档Ingestion + 事件监听
│
├── internal/
│   ├── common/
│   │   ├── middleware/               # 鉴权、限流、 tracing、recovery
│   │   └── cache/                    # Redis封装（Redigo / go-redis）
│   │
│   ├── knowledge/                    # Bounded Context 1：知识管理域（RAG 核心）
│   │   ├── domain/                   # 领域层（纯业务实体 + 不变式）
│   │   │   ├── chunk.go              
│   │   │   ├── document.go           
│   │   │   ├── embedding.go
│   │   │   └── repository.go         # 仓储接口        
│   │   ├── application/              # 用例层（协调领域）
│   │   │   ├── usecase/              # IngestUseCase、RetrieveUseCase、SearchUseCase
│   │   │       ├── ingestion/
│   │   │       └── retrieval/
│   │   │   └── dto/                  # 输入输出DTO（防止领域泄漏）
│   │   ├── chunking/                 # Chunking 策略实现（已统一）
│   │   │   ├── chunking.go           # 接口定义
│   │   │   ├── recursive_character.go
│   │   │   ├── sliding_window.go
│   │   │   └── semantic.go          
│   │   ├── infrastructure/           # 基础设施适配器
│   │   │   ├── milvus/               
│   │   │   ├── elasticsearch/        
│   │   │   └── embedding/            # OpenAI/Qwen/Claude 等 Embedding Client
│   │   └── repository/               # 端口（接口定义）—— 依赖倒置核心
│   │
│   ├── workflow/                     # Bounded Context 2：DAG + 多Agent编排域
│   │   ├── domain/
│   │   │   ├── entity/               # DAGNode、Tool、Agent 等
│   │   │   ├── aggregate/            # Workflow（聚合根 + 无环不变式）
│   │   │   ├── repository/           
│   │   │   └── service/              # AgentPlannerService（ReAct/ Plan-and-Execute）
│   │   ├── application/
│   │   │   └── usecase/              # ExecuteWorkflowUseCase、TriggerWorkflowUseCase
│   │   └── infrastructure/
│   │       ├── dag/                  # DAGExecutor（errgroup + 拓扑排序）
│   │       └── tools/                # ToolRegistry + 具体 Tool 适配器（API、DB、自定义）
│   │
│   ├── gateway/                      # Bounded Context 3：LLM代理网关域（成本控制核心）
│   │   ├── domain/
│   │   │   ├── entity/               # Tenant、RateLimitRule、BillingPolicy
│   │   │   └── service/              # TokenBillingService
│   │   ├── application/
│   │   │   └── usecase/              # ProxyUseCase（统一入口）
│   │   └── infrastructure/
│   │       ├── redis/                # RateLimiter + Semantic Cache
│   │       └── jwt/                  # 多租户 JWT 鉴权
│   │
│   ├── shared/                       # 共享内核（所有Context公用）
│   │   ├── kernel/                   # 领域通用工具
│   │   │   ├── errors.go             # 领域错误（自定义 error 类型）
│   │   │   ├── events.go             # 领域事件接口 + Watermill
│   │   │   ├── uuid.go               # ID生成
│   │   │   └── validator.go          # 验证器
│   │   ├── types/                    # 公共类型、常量、枚举
│   │   └── logger/                   # zap封装+ 结构化日志
│   │
│   └── config/                       # 配置加载（Viper + 多环境）
│       └── config.go                 
├── pkg/                              # 极少使用，仅放跨项目工具库
├── deployments/                      # 部署相关
│   ├── docker-compose.yml            # Docker Compose 配置
│   └── k8s/                          # Kubernetes 配置
├── go.mod
├── go.sum
├── README.md                     # 必须包含完整目录说明 + 启动命令
└── Makefile                      # 常用命令（build、test、migrate、lint）
```