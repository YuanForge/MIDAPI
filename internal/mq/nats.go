package mq

import (
	"fmt"
	"log"
	"strings"
	"time"
	"unicode"

	"fanapi/internal/config"

	"github.com/nats-io/nats.go"
)

const (
	defaultNamespace = "master"

	// workerAckWait: worker must ACK within this window or the message is redelivered.
	// Set to well above the longest expected task processing time.
	workerAckWait = 10 * time.Minute

	// workerMaxDeliver: max delivery attempts before the message is dropped (not retried forever).
	workerMaxDeliver = 3
)

var (
	Conn  *nats.Conn
	JS    nats.JetStreamContext
	mqCfg natsRuntimeConfig
)

func Init(cfg *config.NATSConfig) error {
	mqCfg = normalizeRuntimeConfig(cfg)
	var err error
	Conn, err = nats.Connect(mqCfg.URL)
	if err != nil {
		return fmt.Errorf("nats connect: %w", err)
	}
	JS, err = Conn.JetStream()
	if err != nil {
		return fmt.Errorf("jetstream context: %w", err)
	}
	return nil
}

type natsRuntimeConfig struct {
	URL           string
	Namespace     string
	TaskStream    string
	TaskSubject   string
	ResultStream  string
	ResultSubject string
	MemoryStorage bool
	Replicas      int
}

func normalizeRuntimeConfig(cfg *config.NATSConfig) natsRuntimeConfig {
	rt := natsRuntimeConfig{
		Namespace: defaultNamespace,
		Replicas:  1,
	}
	if cfg != nil {
		rt.URL = strings.TrimSpace(cfg.URL)
		rt.Namespace = sanitizeName(cfg.Namespace)
		rt.TaskStream = strings.TrimSpace(cfg.TaskStream)
		rt.TaskSubject = strings.TrimSpace(cfg.TaskSubject)
		rt.ResultStream = strings.TrimSpace(cfg.ResultStream)
		rt.ResultSubject = strings.TrimSpace(cfg.ResultSubject)
		rt.MemoryStorage = cfg.MemoryStorage
		if cfg.Replicas > 1 {
			rt.Replicas = cfg.Replicas
		}
	}
	if rt.Namespace == "" {
		rt.Namespace = defaultNamespace
	}
	if rt.TaskStream == "" {
		rt.TaskStream = "TASKS_" + rt.Namespace
	}
	if rt.TaskSubject == "" {
		rt.TaskSubject = rt.Namespace + ".task.>"
	}
	if rt.ResultStream == "" {
		rt.ResultStream = "RESULTS_" + rt.Namespace
	}
	if rt.ResultSubject == "" {
		rt.ResultSubject = rt.Namespace + ".result.>"
	}
	return rt
}

// Namespace returns the current logical NATS namespace.
func Namespace() string {
	return mqCfg.Namespace
}

func TaskStreamName() string {
	return mqCfg.TaskStream
}

func ResultStreamName() string {
	return mqCfg.ResultStream
}

func TaskSubjectPattern() string {
	return mqCfg.TaskSubject
}

func ResultSubjectPattern() string {
	return mqCfg.ResultSubject
}

// TaskSubject builds a concrete task subject inside the current namespace.
func TaskSubject(taskType string, channelID int64) string {
	return fmt.Sprintf("%s.%s.%d", subjectPrefix(mqCfg.TaskSubject), taskType, channelID)
}

// ResultSubject builds a concrete result subject inside the current namespace.
func ResultSubject(taskID int64) string {
	return fmt.Sprintf("%s.%d", subjectPrefix(mqCfg.ResultSubject), taskID)
}

// NormalizeTaskSubscription scopes old worker subjects like "task.video.*" into
// the current namespace. Fully qualified subjects are left untouched.
func NormalizeTaskSubscription(subject string) string {
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return TaskSubjectPattern()
	}
	prefix := subjectPrefix(mqCfg.TaskSubject)
	if subject == prefix || strings.HasPrefix(subject, prefix+".") {
		return subject
	}
	if subject == "task" || strings.HasPrefix(subject, "task.") {
		suffix := strings.TrimPrefix(subject, "task")
		return prefix + suffix
	}
	return subject
}

func ConsumerName(parts ...string) string {
	items := make([]string, 0, len(parts)+1)
	items = append(items, mqCfg.Namespace)
	items = append(items, parts...)
	return sanitizeName(strings.Join(items, "-"))
}

func subjectPrefix(pattern string) string {
	pattern = strings.TrimSpace(pattern)
	pattern = strings.TrimSuffix(pattern, ".>")
	pattern = strings.TrimSuffix(pattern, ".*")
	pattern = strings.TrimSuffix(pattern, ">")
	pattern = strings.TrimSuffix(pattern, "*")
	return strings.TrimSuffix(pattern, ".")
}

func sanitizeName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(unicode.ToLower(r))
			lastDash = false
		case r == '_' || r == '-':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-_")
}

// EnsureStream creates or updates the persistent task and result JetStream streams.
// Must be called once on startup in every process that uses NATS.
func EnsureStream() error {
	storage := nats.FileStorage
	if mqCfg.MemoryStorage {
		storage = nats.MemoryStorage
	}
	if err := ensureOneStream(&nats.StreamConfig{
		Name:      mqCfg.TaskStream,
		Subjects:  []string{mqCfg.TaskSubject},
		Retention: nats.WorkQueuePolicy,
		Storage:   storage,
		MaxAge:    24 * time.Hour,
		Replicas:  mqCfg.Replicas,
	}); err != nil {
		return err
	}
	return ensureOneStream(&nats.StreamConfig{
		Name:      mqCfg.ResultStream,
		Subjects:  []string{mqCfg.ResultSubject},
		Retention: nats.WorkQueuePolicy,
		Storage:   storage,
		MaxAge:    24 * time.Hour,
		Replicas:  mqCfg.Replicas,
	})
}

func ensureOneStream(cfg *nats.StreamConfig) error {
	_, err := JS.StreamInfo(cfg.Name)
	if err == nats.ErrStreamNotFound {
		if _, err = JS.AddStream(cfg); err != nil {
			return fmt.Errorf("create %s stream: %w", cfg.Name, err)
		}
		log.Printf("[mq] JetStream stream %q created", cfg.Name)
	} else if err != nil {
		return fmt.Errorf("stream info (%s): %w", cfg.Name, err)
	} else {
		if _, err = JS.UpdateStream(cfg); err != nil {
			return fmt.Errorf("update %s stream: %w", cfg.Name, err)
		}
		log.Printf("[mq] JetStream stream %q confirmed", cfg.Name)
	}
	return nil
}

// PublishResult durably publishes a worker result to the configured result stream.
func PublishResult(subject string, data []byte) error {
	_, err := JS.Publish(subject, data)
	return err
}

// Publish persists a message durably to JetStream before returning.
// The call blocks until the NATS server acknowledges the write to the stream.
func Publish(subject string, data []byte) error {
	_, err := JS.Publish(subject, data)
	return err
}

// QueueSubscribe creates a durable JetStream pull consumer and starts a background goroutine
// to continuously fetch messages. Pull consumers work correctly on WorkQueue streams and
// survive worker restarts without the "filtered consumer not unique" error that push
// consumers suffer on WorkQueue streams.
//
// maxConcurrent limits the number of goroutines running handler concurrently.
// When the limit is reached, the fetch loop blocks — providing true backpressure:
// no new messages are pulled from the queue until a slot is freed.
// Pass 0 for unlimited concurrency.
func QueueSubscribe(subject, queue string, handler nats.MsgHandler, maxConcurrent int) (*nats.Subscription, error) {
	sub, err := JS.PullSubscribe(
		subject,
		queue,
		nats.AckExplicit(),
		nats.AckWait(workerAckWait),
		nats.MaxDeliver(workerMaxDeliver),
	)
	if err != nil {
		return nil, fmt.Errorf("pull subscribe %s/%s: %w", subject, queue, err)
	}

	var sem chan struct{}
	if maxConcurrent > 0 {
		sem = make(chan struct{}, maxConcurrent)
	}

	go func() {
		for {
			msgs, fetchErr := sub.Fetch(10, nats.MaxWait(5*time.Second))
			if fetchErr != nil {
				if !sub.IsValid() {
					return // subscription closed, exit cleanly
				}
				// ErrTimeout is normal when the queue is empty
				if fetchErr != nats.ErrTimeout {
					log.Printf("[mq] fetch error (%s): %v", subject, fetchErr)
					time.Sleep(time.Second)
				}
				continue
			}
			for _, msg := range msgs {
				if sem != nil {
					// 先占槽再 spawn goroutine：fetch 循环在此阻塞，
					// 达到上限时不再从队列拉取新消息，形成真正的背压。
					sem <- struct{}{}
				}
				go func(m *nats.Msg) {
					if sem != nil {
						defer func() { <-sem }()
					}
					handler(m)
				}(msg)
			}
		}
	}()

	return sub, nil
}

// Subscribe creates a core NATS subscription (non-persistent, for ancillary use cases).
func Subscribe(subject string, handler nats.MsgHandler) (*nats.Subscription, error) {
	return Conn.Subscribe(subject, handler)
}

// PurgeConsumers removes the named consumers from the configured task stream.
// Must be called once from the worker process before QueueSubscribe to clear stale
// consumers left over from previous runs (prevents "filtered consumer not unique"
// on WorkQueue streams). Must NOT be called from the server process.
func PurgeConsumers(names ...string) {
	if len(names) == 0 {
		log.Printf("[mq] no consumers requested for purge on stream %s", mqCfg.TaskStream)
		return
	}
	wanted := make(map[string]struct{}, len(names))
	for _, name := range names {
		if name = strings.TrimSpace(name); name != "" {
			wanted[name] = struct{}{}
		}
	}
	if len(wanted) == 0 {
		return
	}
	for info := range JS.Consumers(mqCfg.TaskStream) {
		if _, ok := wanted[info.Name]; !ok {
			continue
		}
		if delErr := JS.DeleteConsumer(mqCfg.TaskStream, info.Name); delErr == nil {
			log.Printf("[mq] purged stale consumer %q from stream %s", info.Name, mqCfg.TaskStream)
		}
	}
}
