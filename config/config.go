package config

import (
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/kelseyhightower/envconfig"
	"gitlab.com/thorchain/midgard/internal/util/midlog"
)

type Duration time.Duration

type Config struct {
	ListenPort      int      `json:"listen_port" split_words:"true"`
	ShutdownTimeout Duration `json:"shutdown_timeout" split_words:"true"`

	// ReadTimeout and WriteTimeout refer to the webserver timeouts
	ReadTimeout  Duration `json:"read_timeout" split_words:"true"`
	WriteTimeout Duration `json:"write_timeout" split_words:"true"`

	// v2/health:InSync is true if Now - LastAvailableBlock < MaxBlockAge
	MaxBlockAge Duration `json:"max_block_age" split_words:"true"`

	ThorChain ThorChain `json:"thorchain"`

	BlockStore BlockStore

	// TODO(muninn): Renaming this to DB whenever config values are renamed in coordination with SREs.
	TimeScale TimeScale `json:"timescale"`

	Websockets Websockets `json:"websockets" split_words:"true"`

	UsdPools []string `json:"usdpools" split_words:"true"`

	EventRecorder EventRecorder `json:"event_recorder" split_words:"true"`

	CaseInsensitiveChains map[string]bool `json:"case_insensitive_chains" split_words:"true"`

	Logs midlog.LogConfig `json:"logs" split_words:"true"`
}

type BlockStore struct {
	Local                  string `json:"local" split_words:"true"`
	Remote                 string `json:"remote" split_words:"true"`
	BlocksPerChunk         int64  `json:"blocks_per_chunk" split_words:"true"`
	CompressionLevel       int    `json:"compression_level" split_words:"true"`
	ChunkHashesPath        string `json:"chunk_hashes_path" split_words:"true"`
	DownloadFullChunksOnly bool   `json:"download_full_chunks_only" split_words:"true"`
}

type EventRecorder struct {
	OnTransferEnabled bool `json:"on_transfer_enabled" split_words:"true"`
	OnMessageEnabled  bool `json:"on_message_enabled" split_words:"true"`
}

type ThorChain struct {
	TendermintURL               string   `json:"tendermint_url" split_words:"true"`
	ThorNodeURL                 string   `json:"thornode_url" split_words:"true"`
	ProxiedWhitelistedEndpoints []string `json:"proxied_whitelisted_endpoints" split_words:"true"`
	FetchBatchSize              int      `json:"fetch_batch_size" split_words:"true"`
	Parallelism                 int      `json:"parallelism" split_words:"true"`

	// Timeout for fetch requests to ThorNode
	ReadTimeout Duration `json:"read_timeout" split_words:"true"`

	// If fetch from ThorNode fails, wait this much before retrying
	LastChainBackoff Duration `json:"last_chain_backoff" split_words:"true"`

	// Entries found in the config are appended to the compiled-in entries from `chainancestry.go`
	// (ie., they override the compiled-in values if there is a definition for the same ChainId
	// in both.)
	//
	// Parent chains should come before their children.
	ForkInfos []ForkInfo `json:"fork_infos" split_words:"true"`
}

// Both `EarliestBlockHash` and `EarliestBlockHeight` are optional and mostly just used for sanity
// checking.
//
// `EarliestBlockHeight` defaults to 1, if it has no parent. Or to `parent.HardForkHeight + 1`
// otherwise.
//
// If `EarliestBlockHash` is unset then its consistency with DB is not checked.
//
// If `HardForkHeight` is set for the current chain Midgard will stop there. This height will be
// the last written to the DB.
//
// When a fork is coming up it's useful to prevent Midgard from writing out data from the old chain
// beyond the fork height.
type ForkInfo struct {
	ChainId             string `json:"chain_id" split_words:"true"`
	ParentChainId       string `json:"parent_chain_id" split_words:"true"`
	EarliestBlockHash   string `json:"earliest_block_hash" split_words:"true"`
	EarliestBlockHeight int64  `json:"earliest_block_height" split_words:"true"`
	HardForkHeight      int64  `json:"hard_fork_height" split_words:"true"`
}

type TimeScale struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	UserName string `json:"user_name"`
	Password string `json:"password"`
	Database string `json:"database"`
	Sslmode  string `json:"sslmode"`

	// -1 sets it to infinite
	MaxOpenConns    int `json:"max_open_conns"`
	CommitBatchSize int `json:"commit_batch_size"`

	// If DDL mismatch is detected exit with error instead of resetting the schema
	NoAutoUpdateDDL bool `json:"no_auto_update_ddl"`
	// If DDL mismatch for aggregates is detected exit with error instead of resetting
	// the aggregates. Implies `NoAutoUpdateDDL`
	NoAutoUpdateAggregatesDDL bool `json:"no_auto_update_aggregates_ddl"`
}

type Websockets struct {
	Enable          bool `json:"enable" split_words:"true"`
	ConnectionLimit int  `json:"connection_limit" split_words:"true"`
}

var defaultConfig = Config{
	ListenPort: 8080,
	ThorChain: ThorChain{
		ThorNodeURL:      "http://localhost:1317/thorchain",
		TendermintURL:    "http://localhost:26657/websocket",
		ReadTimeout:      Duration(8 * time.Second),
		LastChainBackoff: Duration(7 * time.Second),

		// NOTE(huginn): numbers are chosen to give a good performance on an "average" desktop
		// machine with a 4 core CPU. With more cores it might make sense to increase the
		// parallelism, though care should be taken to not overload the Thornode.
		// See `docs/parallel_batch_bench.md` for measurments to guide selection of these parameters.
		FetchBatchSize: 100, // must be divisible by BlockFetchParallelism
		Parallelism:    4,
	},
	BlockStore: BlockStore{
		BlocksPerChunk:   10000,
		CompressionLevel: 1, // 0 means no compression
	},
	TimeScale: TimeScale{
		MaxOpenConns:    80,
		CommitBatchSize: 100,
	},
	ShutdownTimeout: Duration(20 * time.Second),
	ReadTimeout:     Duration(20 * time.Second),
	WriteTimeout:    Duration(20 * time.Second),
	MaxBlockAge:     Duration(60 * time.Second),
	UsdPools: []string{
		"BNB.BUSD-BD1",
		"ETH.USDT-0XDAC17F958D2EE523A2206206994597C13D831EC7",
		"ETH.USDC-0XA0B86991C6218B36C1D19D4A2E9EB0CE3606EB48",
	},
	CaseInsensitiveChains: map[string]bool{
		"ETH": true,
	},
	EventRecorder: EventRecorder{
		OnTransferEnabled: false,
		OnMessageEnabled:  false,
	},
}

var logger = midlog.LoggerForModule("config")

func (d Duration) Value() time.Duration {
	return time.Duration(d)
}
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch value := v.(type) {
	case string:
		v, err := time.ParseDuration(value)
		if err != nil {
			return err
		}
		*d = Duration(v)
	default:
		return errors.New("duration not a string")
	}
	return nil
}

func (d *Duration) Decode(value string) error {
	v, err := time.ParseDuration(value)
	if err != nil {
		return err
	}
	*d = Duration(v)
	return nil
}

func MustLoadConfigFiles(colonSeparatedFilenames string, c *Config) {
	if colonSeparatedFilenames == "" || colonSeparatedFilenames == "null" {
		return
	}

	for _, filename := range strings.Split(colonSeparatedFilenames, ":") {
		mustLoadConfigFile(filename, c)
	}
}

func mustLoadConfigFile(path string, c *Config) {
	f, err := os.Open(path)
	if err != nil {
		logger.FatalE(err, "Exit on configuration file unavailable")
	}
	defer f.Close()

	dec := json.NewDecoder(f)

	// prevent config not used due typos
	dec.DisallowUnknownFields()

	if err := dec.Decode(&c); err != nil {
		logger.FatalE(err, "Exit on malformed configuration")
	}
}

func LogAndcheckUrls(c *Config) {
	urls := []struct {
		url, name string
	}{
		{c.ThorChain.ThorNodeURL, "THORNode REST URL"},
		{c.ThorChain.TendermintURL, "Tendermint RPC URL"},
		{c.BlockStore.Remote, "BlockStore Remote URL"},
	}
	for _, v := range urls {
		logger.InfoF("%s: %q", v.name, v.url)
		if _, err := url.Parse(v.url); err != nil {
			logger.FatalEF(err, "Exit on malformed %s", v.url)
		}
	}
}

// Not thread safe, it is written once, then only read
var Global Config = defaultConfig

func readConfigFrom(filenames string) Config {
	var ret Config = defaultConfig
	MustLoadConfigFiles(filenames, &ret)

	// override config with env variables
	err := envconfig.Process("midgard", &ret)
	if err != nil {
		logger.FatalE(err, "Failed to process config environment variables")
	}

	LogAndcheckUrls(&ret)

	return ret
}

// filenames is a colon separated list of files.
// Values in later files overwrite values from earlier files.
func ReadGlobalFrom(filenames string) {
	Global = readConfigFrom(filenames)
	midlog.SetFromConfig(Global.Logs)
}

func ReadGlobal() {
	switch len(os.Args) {
	case 1:
		ReadGlobalFrom("")
	case 2:
		ReadGlobalFrom(os.Args[1])
	default:
		logger.Fatal("One optional configuration file argument only-no flags")
	}
}
