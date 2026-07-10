package config

// Config 描述整个后端的顶层配置结构，并直接映射到 YAML 文件。
type Config struct {
	Name               string                   `yaml:"Name"`
	Host               string                   `yaml:"Host"`
	Port               int                      `yaml:"Port"`
	HTTP               HTTPConfig               `yaml:"HTTP"`
	GRPC               GRPCConfig               `yaml:"GRPC"`
	Video              VideoConfig              `yaml:"Video"`
	FFmpeg             FFmpegConfig             `yaml:"FFmpeg"`
	Storage            StorageConfig            `yaml:"Storage"`
	Redis              RedisConfig              `yaml:"Redis"`
	RedisKeys          RedisKeysConfig          `yaml:"RedisKeys"`
	Postgres           PostgresConfig           `yaml:"Postgres"`
	RustFS             RustFSConfig             `yaml:"RustFS"`
	Transcode          TransConfig              `yaml:"Transcode"`
	VectorWorker       VectorWorkerConfig       `yaml:"VectorWorker"`
	VectorStageWorkers VectorStageWorkersConfig `yaml:"VectorStageWorkers"`
	WorkerPools        WorkerPoolsConfig        `yaml:"WorkerPools"`
	Recommendation     RecommendationConfig     `yaml:"Recommendation"`
	Gorse              GorseConfig              `yaml:"Gorse"`
	Embedding          EmbeddingConfig          `yaml:"embedding"`
	ASR                ASRConfig                `yaml:"asr"`
	AI                 AIConfig                 `yaml:"AI"`
}

// HTTPConfig 定义 HTTP API 监听地址。
type HTTPConfig struct {
	Addr               string     `yaml:"Addr"`
	ShutdownTimeoutSec int        `yaml:"ShutdownTimeoutSec"`
	LogDir             string     `yaml:"LogDir"`
	SlowRequestMs      int        `yaml:"SlowRequestMs"`
	CORS               CORSConfig `yaml:"CORS"`
}

// CORSConfig 定义浏览器跨域响应头。
type CORSConfig struct {
	AllowOrigin   string `yaml:"AllowOrigin"`
	AllowMethods  string `yaml:"AllowMethods"`
	AllowHeaders  string `yaml:"AllowHeaders"`
	ExposeHeaders string `yaml:"ExposeHeaders"`
	MaxAge        string `yaml:"MaxAge"`
}

// GRPCConfig 定义 gRPC 服务端的消息大小与连接生命周期参数。
type GRPCConfig struct {
	MaxMsgSize            int `yaml:"MaxMsgSize"`
	KeepaliveTime         int `yaml:"KeepaliveTime"`
	KeepaliveTimeout      int `yaml:"KeepaliveTimeout"`
	MaxConnectionAge      int `yaml:"MaxConnectionAge"`
	MaxConnectionAgeGrace int `yaml:"MaxConnectionAgeGrace"`
}

// VideoConfig 保存本地视频目录相关配置。
type VideoConfig struct {
	RawPath string `yaml:"RawPath"`
	HlsPath string `yaml:"HlsPath"`
}

// StorageConfig 定义对象存储 key 与对外访问 URL 前缀。
type StorageConfig struct {
	RawObjectPrefix  string `yaml:"RawObjectPrefix"`
	HLSObjectPrefix  string `yaml:"HLSObjectPrefix"`
	MediaRoutePrefix string `yaml:"MediaRoutePrefix"`
	RawURLPrefix     string `yaml:"RawURLPrefix"`
	HLSURLPrefix     string `yaml:"HLSURLPrefix"`
	CoverURLPrefix   string `yaml:"CoverURLPrefix"`
	VectorTempPath   string `yaml:"VectorTempPath"`
}

// FFmpegConfig 汇总转码、截图、音频提取等 FFmpeg 相关配置。
type FFmpegConfig struct {
	UseDocker   bool              `yaml:"UseDocker"`
	DockerImage string            `yaml:"DockerImage"`
	HLS         FFmpegHLSConfig   `yaml:"HLS"`
	Fast        FFmpegFastConfig  `yaml:"Fast"`
	Cover       FFmpegCoverConfig `yaml:"Cover"`
	Audio       FFmpegAudioConfig `yaml:"Audio"`
}

// FFmpegHLSConfig 定义 HLS 切片输出的关键参数。
type FFmpegHLSConfig struct {
	Time           int    `yaml:"Time"`
	ListSize       int    `yaml:"ListSize"`
	MasterName     string `yaml:"MasterName"`
	SegmentPattern string `yaml:"SegmentPattern"`
}

// FFmpegFastConfig 定义快速转码路径的编码参数。
type FFmpegFastConfig struct {
	ScaleW        int    `yaml:"ScaleW"`
	ScaleH        int    `yaml:"ScaleH"`
	Preset        string `yaml:"Preset"`
	Crf           int    `yaml:"Crf"`
	PixFmt        string `yaml:"PixFmt"`
	AudioBitrate  string `yaml:"AudioBitrate"`
	AudioChannels int    `yaml:"AudioChannels"`
	PadToFit      bool   `yaml:"PadToFit"`
}

// FFmpegCoverConfig 定义封面截图时的取帧策略。
type FFmpegCoverConfig struct {
	SeekSec         int `yaml:"SeekSec"`
	FallbackSeekSec int `yaml:"FallbackSeekSec"`
	Quality         int `yaml:"Quality"`
}

// FFmpegAudioConfig 定义抽取音频时的采样率与声道数。
type FFmpegAudioConfig struct {
	SampleRate int `yaml:"SampleRate"`
	Channels   int `yaml:"Channels"`
}

// RedisConfig 定义 Redis 连接信息。
type RedisConfig struct {
	Addr     string `yaml:"Addr"`
	Password string `yaml:"Password"`
	DB       int    `yaml:"DB"`
}

// RedisKeysConfig 定义 Redis 队列、状态和运行计数 key 前缀。
type RedisKeysConfig struct {
	TranscodeQueue        string `yaml:"TranscodeQueue"`
	VectorizeQueue        string `yaml:"VectorizeQueue"`
	VectorPrepareQueue    string `yaml:"VectorPrepareQueue"`
	VectorCoarseQueue     string `yaml:"VectorCoarseQueue"`
	VectorRefineQueue     string `yaml:"VectorRefineQueue"`
	VectorFinalizeQueue   string `yaml:"VectorFinalizeQueue"`
	VideoReactionQueue    string `yaml:"VideoReactionQueue"`
	VideoReactionCounts   string `yaml:"VideoReactionCounts"`
	VideoReactionUser     string `yaml:"VideoReactionUser"`
	SegmentReactionQueue  string `yaml:"SegmentReactionQueue"`
	SegmentReactionCounts string `yaml:"SegmentReactionCounts"`
	SegmentReactionUser   string `yaml:"SegmentReactionUser"`
	TranscodeStatus       string `yaml:"TranscodeStatus"`
	RuntimeActiveCounter  string `yaml:"RuntimeActiveCounter"`
	RandomPlayRecent      string `yaml:"RandomPlayRecent"`
	RandomPlayBucket      string `yaml:"RandomPlayBucket"`
}

// PostgresConfig 定义 PostgreSQL 连接信息与连接池参数。
type PostgresConfig struct {
	DSN             string `yaml:"DSN"`
	MaxOpenConns    int    `yaml:"MaxOpenConns"`
	MaxIdleConns    int    `yaml:"MaxIdleConns"`
	ConnMaxLifetime int    `yaml:"ConnMaxLifetime"`
	ConnMaxIdleTime int    `yaml:"ConnMaxIdleTime"`
}

// RustFSConfig 定义对象存储连接参数。
type RustFSConfig struct {
	Endpoint     string `yaml:"Endpoint"`
	AccessKey    string `yaml:"AccessKey"`
	SecretKey    string `yaml:"SecretKey"`
	Bucket       string `yaml:"Bucket"`
	UseSSL       bool   `yaml:"UseSSL"`
	Region       string `yaml:"Region"`
	BucketLookup string `yaml:"BucketLookup"`
}

// TransConfig 定义转码 worker 的并发、超时与模式参数。
type TransConfig struct {
	WorkerCount        int    `yaml:"WorkerCount"`
	QueueSize          int    `yaml:"QueueSize"`
	Mode               string `yaml:"Mode"`
	TaskTimeoutMinutes int    `yaml:"TaskTimeoutMinutes"`
	ShutdownTimeoutSec int    `yaml:"ShutdownTimeoutSec"`
}

// VectorWorkerConfig 定义向量化 worker 的粗分段、细分段、并发和 LLM 参数。
type VectorWorkerConfig struct {
	Mode                       string `yaml:"Mode"`
	CoarseSegmentSec           int    `yaml:"CoarseSegmentSec"`
	RefineMinSegmentSec        int    `yaml:"RefineMinSegmentSec"`
	RefineMaxSegmentSec        int    `yaml:"RefineMaxSegmentSec"`
	LLMModel                   string `yaml:"LLMModel"`
	LLMTimeoutMinutes          int    `yaml:"LLMTimeoutMinutes"`
	TailAlignmentEnabled       bool   `yaml:"TailAlignmentEnabled"`
	TailAlignmentConfigured    bool   `yaml:"TailAlignmentConfigured"`
	TailAlignmentMaxExtendSec  int    `yaml:"TailAlignmentMaxExtendSec"`
	TailAlignmentProbeStepSec  int    `yaml:"TailAlignmentProbeStepSec"`
	TailAlignmentMaxOverlapSec int    `yaml:"TailAlignmentMaxOverlapSec"`

	SegmentWindowSec   int `yaml:"SegmentWindowSec"`
	SegmentStepSec     int `yaml:"SegmentStepSec"`
	ASRWorkers         int `yaml:"ASRWorkers"`
	CoarseWorkers      int `yaml:"CoarseWorkers"`
	EmbedBatch         int `yaml:"EmbedBatch"`
	SampleCount        int `yaml:"SampleCount"`
	SampleDurSec       int `yaml:"SampleDurSec"`
	TaskTimeoutMinutes int `yaml:"TaskTimeoutMinutes"`
	ShutdownTimeoutSec int `yaml:"ShutdownTimeoutSec"`
}

type VectorStageWorkersConfig struct {
	Prepare  int `yaml:"Prepare"`
	Coarse   int `yaml:"Coarse"`
	Refine   int `yaml:"Refine"`
	Finalize int `yaml:"Finalize"`
}

type WorkerPoolsConfig map[string]WorkerPoolConfig

type WorkerPoolConfig struct {
	Size int `yaml:"Size"`
}

// RecommendationConfig controls which personalized recommendation chain is primary.
type RecommendationConfig struct {
	Engine                    string `yaml:"Engine"`
	RandomPlayDedupeWindowSec int    `yaml:"RandomPlayDedupeWindowSec"`
	RandomPlayRecentMaxSize   int    `yaml:"RandomPlayRecentMaxSize"`
}

// GorseConfig defines the local Gorse recommendation engine integration.
type GorseConfig struct {
	Endpoint          string `yaml:"Endpoint"`
	APIKey            string `yaml:"APIKey"`
	TimeoutSeconds    int    `yaml:"TimeoutSeconds"`
	ShadowMode        bool   `yaml:"ShadowMode"`
	SyncEnabled       bool   `yaml:"SyncEnabled"`
	WriteBackEnabled  bool   `yaml:"WriteBackEnabled"`
	CandidateLimit    int    `yaml:"CandidateLimit"`
	SyncIntervalMins  int    `yaml:"SyncIntervalMins"`
	EnableGate        bool   `yaml:"EnableGate"`
	MinFeedbackCount  int    `yaml:"MinFeedbackCount"`
	MinRecommendItems int    `yaml:"MinRecommendItems"`
	CleanupEnabled    bool   `yaml:"CleanupEnabled"`
	DataRetentionDays int    `yaml:"DataRetentionDays"`
}

// EmbeddingConfig 定义向量化所需的 Embedding 服务配置。
type EmbeddingConfig struct {
	Options struct {
		Model string `yaml:"model"`
	} `yaml:"options"`
	BaseURL string `yaml:"base-url"`
	APIKey  string `yaml:"api-key"`
}

// ASRConfig 定义语音识别服务及其 WebSocket 回退模型配置。
type ASRConfig struct {
	Options struct {
		Model       string   `yaml:"model"`
		WSModel     string   `yaml:"ws-model"`
		WSFallbacks []string `yaml:"ws-model-fallbacks"`
	} `yaml:"options"`
	BaseURL string `yaml:"base-url"`
	WSURL   string `yaml:"ws-url"`
	APIKey  string `yaml:"api-key"`
}

// AIConfig 保存向量维度等 AI 基础参数。
type AIConfig struct {
	EmbeddingDim int    `yaml:"EmbeddingDim"`
	Provider     string `yaml:"Provider"`
}
