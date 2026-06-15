package config

// Config 描述整个后端的顶层配置结构，并直接映射到 YAML 文件。
type Config struct {
	Name         string             `yaml:"Name"`
	Host         string             `yaml:"Host"`
	Port         int                `yaml:"Port"`
	GRPC         GRPCConfig         `yaml:"GRPC"`
	Video        VideoConfig        `yaml:"Video"`
	FFmpeg       FFmpegConfig       `yaml:"FFmpeg"`
	Redis        RedisConfig        `yaml:"Redis"`
	Postgres     PostgresConfig     `yaml:"Postgres"`
	RustFS       RustFSConfig       `yaml:"RustFS"`
	Transcode    TransConfig        `yaml:"Transcode"`
	VectorWorker VectorWorkerConfig `yaml:"VectorWorker"`
	WorkerPools  WorkerPoolsConfig  `yaml:"WorkerPools"`
	Embedding    EmbeddingConfig    `yaml:"embedding"`
	ASR          ASRConfig          `yaml:"asr"`
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
	Endpoint  string `yaml:"Endpoint"`
	AccessKey string `yaml:"AccessKey"`
	SecretKey string `yaml:"SecretKey"`
	Bucket    string `yaml:"Bucket"`
	UseSSL    bool   `yaml:"UseSSL"`
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
	Mode                      string `yaml:"Mode"`
	CoarseSegmentSec          int    `yaml:"CoarseSegmentSec"`
	RefineMinSegmentSec       int    `yaml:"RefineMinSegmentSec"`
	RefineMaxSegmentSec       int    `yaml:"RefineMaxSegmentSec"`
	LLMModel                  string `yaml:"LLMModel"`
	LLMTimeoutMinutes         int    `yaml:"LLMTimeoutMinutes"`
	TailAlignmentEnabled      bool   `yaml:"TailAlignmentEnabled"`
	TailAlignmentConfigured   bool   `yaml:"TailAlignmentConfigured"`
	TailAlignmentMaxExtendSec int    `yaml:"TailAlignmentMaxExtendSec"`
	TailAlignmentProbeStepSec int    `yaml:"TailAlignmentProbeStepSec"`
	TailAlignmentMaxOverlapSec int   `yaml:"TailAlignmentMaxOverlapSec"`

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

// WorkerPoolsConfig 定义具名协程池的通用配置入口。
type WorkerPoolsConfig map[string]WorkerPoolConfig

// WorkerPoolConfig 定义单个具名协程池的配置。
type WorkerPoolConfig struct {
	Size int `yaml:"Size"`
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
		Model   string `yaml:"model"`
		WSModel string `yaml:"ws-model"`
	} `yaml:"options"`
	BaseURL string `yaml:"base-url"`
	APIKey  string `yaml:"api-key"`
}
