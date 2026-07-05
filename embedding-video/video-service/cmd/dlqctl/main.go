package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	goredis "github.com/go-redis/redis/v8"

	"nlp-video-analysis/internal/config"
	infraredis "nlp-video-analysis/internal/infrastructure/redis"
)

type queueSpec struct {
	Name      string
	StreamKey string
}

type runner struct {
	cfg config.Config
	rdb *goredis.Client
}

func main() {
	config.EnsureProjectRoot()
	cfg := config.MustLoadDefault()
	rdb, err := newRedisClient(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer rdb.Close()

	if err := (&runner{cfg: cfg, rdb: rdb}).run(context.Background(), os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRedisClient(cfg config.Config) (*goredis.Client, error) {
	if strings.TrimSpace(cfg.Redis.Addr) == "" {
		return nil, errors.New("redis addr is required")
	}
	return goredis.NewClient(&goredis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	}), nil
}

func (r *runner) run(ctx context.Context, args []string, out io.Writer) error {
	if len(args) == 0 {
		return r.printUsage(out)
	}
	switch args[0] {
	case "list":
		return r.runList(ctx, args[1:], out)
	case "replay":
		return r.runReplay(ctx, args[1:], out)
	case "help", "-h", "--help":
		return r.printUsage(out)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func (r *runner) runList(ctx context.Context, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	queue := fs.String("queue", "all", "queue name")
	limit := fs.Int64("limit", 20, "max dead letters per queue")
	if err := fs.Parse(args); err != nil {
		return err
	}
	specs, err := r.resolveQueueSpecs(*queue)
	if err != nil {
		return err
	}
	for _, spec := range specs {
		entries, err := infraredis.ListDeadLetters(ctx, r.rdb, spec.StreamKey, *limit)
		if err != nil {
			return fmt.Errorf("list %s: %w", spec.Name, err)
		}
		fmt.Fprintf(out, "queue=%s stream=%s dlq=%s count=%d\n", spec.Name, spec.StreamKey, infraredis.DeadLetterStreamKey(spec.StreamKey), len(entries))
		for _, entry := range entries {
			fmt.Fprintf(out, "  id=%s reason=%s payload=%s\n", entry.ID, entry.Reason, summarizePayload(entry.Payload))
		}
	}
	return nil
}

func (r *runner) runReplay(ctx context.Context, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("replay", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	queue := fs.String("queue", "", "queue name")
	id := fs.String("id", "", "dead letter message id")
	limit := fs.Int64("limit", 0, "max dead letters to replay when --id is omitted")
	dryRun := fs.Bool("dry-run", false, "show messages without replaying")
	keepDLQ := fs.Bool("keep-dlq", false, "keep dead letter entry after replay")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*queue) == "" {
		return errors.New("queue is required")
	}
	specs, err := r.resolveQueueSpecs(*queue)
	if err != nil {
		return err
	}
	if *id != "" && len(specs) != 1 {
		return errors.New("--id can only be used with one concrete queue")
	}
	if *id != "" {
		return r.replayOne(ctx, out, specs[0], *id, *dryRun, *keepDLQ)
	}
	if *limit <= 0 {
		return errors.New("--id or positive --limit is required")
	}
	for _, spec := range specs {
		entries, err := infraredis.ListDeadLetters(ctx, r.rdb, spec.StreamKey, *limit)
		if err != nil {
			return fmt.Errorf("list %s: %w", spec.Name, err)
		}
		for _, entry := range entries {
			if err := r.replayOne(ctx, out, spec, entry.ID, *dryRun, *keepDLQ); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *runner) replayOne(ctx context.Context, out io.Writer, spec queueSpec, id string, dryRun bool, keepDLQ bool) error {
	entry, found, err := infraredis.GetDeadLetter(ctx, r.rdb, spec.StreamKey, id)
	if err != nil {
		return fmt.Errorf("get %s %s: %w", spec.Name, id, err)
	}
	if !found {
		return fmt.Errorf("dead letter %q not found in %s", id, infraredis.DeadLetterStreamKey(spec.StreamKey))
	}
	if dryRun {
		fmt.Fprintf(out, "dry-run queue=%s stream=%s id=%s reason=%s payload=%s\n", spec.Name, spec.StreamKey, entry.ID, entry.Reason, summarizePayload(entry.Payload))
		return nil
	}
	replayed, err := infraredis.ReplayDeadLetter(ctx, r.rdb, spec.StreamKey, id, infraredis.ReplayDeadLetterOptions{KeepDeadLetter: keepDLQ})
	if err != nil {
		return fmt.Errorf("replay %s %s: %w", spec.Name, id, err)
	}
	if !replayed {
		return fmt.Errorf("dead letter %q not found in %s", id, infraredis.DeadLetterStreamKey(spec.StreamKey))
	}
	fmt.Fprintf(out, "replayed queue=%s stream=%s id=%s keep_dlq=%v\n", spec.Name, spec.StreamKey, id, keepDLQ)
	return nil
}

func (r *runner) resolveQueueSpecs(name string) ([]queueSpec, error) {
	specMap := queueSpecs(r.cfg)
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("queue is required")
	}
	if name == "all" {
		names := make([]string, 0, len(specMap))
		for specName := range specMap {
			names = append(names, specName)
		}
		sort.Strings(names)
		specs := make([]queueSpec, 0, len(names))
		for _, specName := range names {
			specs = append(specs, specMap[specName])
		}
		return specs, nil
	}
	spec, ok := specMap[name]
	if !ok {
		return nil, fmt.Errorf("unknown queue %q; available: %s", name, strings.Join(sortedQueueNames(specMap), ", "))
	}
	return []queueSpec{spec}, nil
}

func queueSpecs(cfg config.Config) map[string]queueSpec {
	specs := map[string]queueSpec{
		"transcode":        {Name: "transcode", StreamKey: config.TranscodeQueueKey(cfg)},
		"vectorize":        {Name: "vectorize", StreamKey: config.VectorizeQueueKey(cfg)},
		"vector-prepare":   {Name: "vector-prepare", StreamKey: config.VectorPrepareQueueKey(cfg)},
		"vector-coarse":    {Name: "vector-coarse", StreamKey: config.VectorCoarseQueueKey(cfg)},
		"vector-refine":    {Name: "vector-refine", StreamKey: config.VectorRefineQueueKey(cfg)},
		"vector-finalize":  {Name: "vector-finalize", StreamKey: config.VectorFinalizeQueueKey(cfg)},
		"video-reaction":   {Name: "video-reaction", StreamKey: config.VideoReactionQueueKey(cfg)},
		"segment-reaction": {Name: "segment-reaction", StreamKey: config.SegmentReactionQueueKey(cfg)},
	}
	return specs
}

func sortedQueueNames(specs map[string]queueSpec) []string {
	names := make([]string, 0, len(specs))
	for name := range specs {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func summarizePayload(payload string) string {
	payload = strings.TrimSpace(strings.ReplaceAll(payload, "\n", " "))
	if len(payload) <= 200 {
		return payload
	}
	return payload[:200] + "..."
}

func (r *runner) printUsage(out io.Writer) error {
	fmt.Fprintln(out, "Usage:")
	fmt.Fprintln(out, "  dlqctl list --queue <name|all> [--limit 20]")
	fmt.Fprintln(out, "  dlqctl replay --queue <name|all> (--id <message-id> | --limit <n>) [--dry-run] [--keep-dlq]")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Queues:")
	fmt.Fprintln(out, "  "+strings.Join(sortedQueueNames(queueSpecs(r.cfg)), ", "))
	return nil
}
