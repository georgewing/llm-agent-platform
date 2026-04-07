项目结构：
```
llm-platform/
├── cmd/
│   └── api/                          # Interfaces层：Gin路由、控制器（只负责HTTP转换）
│       └── main.go
│   └── worker
│       └── main.go                   # 文档Ingestion后台Worker
├── internal/
│   ├── common/
│   │   ├── middleware/               # 鉴权、限流
│   │   └── cache/                    # Redis封装
│   ├── knowledge/                    # Bounded Context 1：知识管理域（文档、Chunking、检索）
│   │   ├── domain/                   # 核心！业务规则在这里
│   │   │   ├── chunk.go              # Chunk（纯Go struct + 业务方法）
│   │   │   ├── document.go           # Document（纯Go struct + 业务方法）
│   │   │   └── embedding.go          # Embedding（纯Go struct + 业务方法）
│   │   ├── application/              # 用例层（协调领域）
│   │   │   ├── usecase/              # IngestUseCase、RetrieveUseCase
│   │   │   └── dto/                  # 输入输出DTO（不污染领域）
│   │   ├── infrastructure/           # 技术适配器
│   │   │   ├── milvus/               # MilvusAdapter（实现repository接口）
│   │   │   ├── elasticsearch/        # ESAdapter
│   │   │   └── embedding/            # OpenAI/Qwen Embedding客户端
│   │   └── repository/               # 接口定义
│   │
│   ├── workflow/                     # Bounded Context 2：DAG + 多Agent编排域
│   │   ├── domain/
│   │   │   ├── entity/               # DAGNode、Tool、Agent
│   │   │   ├── aggregate/            # Workflow（聚合，包含DAG拓扑 + 业务不变式：无环）
│   │   │   ├── repository/           # WorkflowRepository 接口
│   │   │   └── service/              # AgentPlannerService（ReAct自主规划）
│   │   ├── application/
│   │   │   └── usecase/              # ExecuteWorkflowUseCase
│   │   └── infrastructure/
│   │       ├── dag/                  # DAGExecutor实现（errgroup + graph）
│   │       └── tools/                # ToolRegistry（API/DB查询适配器）
│   │
│   ├── gateway/                      # Bounded Context 3：LLM代理网关域（限流、计费、多租户）
│   │   ├── domain/
│   │   │   ├── entity/               # Tenant、RateLimitRule、BillingPolicy
│   │   │   └── service/              # TokenBillingService
│   │   ├── application/
│   │   │   └── usecase/              # ProxyUseCase
│   │   └── infrastructure/
│   │       ├── redis/                # RateLimiter + Cache
│   │       └── jwt/                  # MultiTenantAuth
│   │
│   ├── shared/                       # 共享内核（DDD推荐，所有Context公用）
│   │   ├── kernel/                   # errors、events、uuid、validator
│   │   ├── types/                    # 公共DTO、常量
│   │   └── logger/                   # zap封装
│   │
│   └── config/                       # 配置加载（Infrastructure层）
├── pkg/                              # 极少使用，仅跨项目工具（DDD倾向放入shared）
├── deployments/                      # Docker + K8s
└── go.mod
```