package vectorworker

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

func normalizeDashscopeWSModel(model string) string {
	raw := strings.TrimSpace(model)
	if raw == "" {
		return "paraformer-realtime-v2"
	}
	m := strings.ToLower(raw)
	switch m {
	case "paraformer-realtime-v2", "paraformer-realtime_v2", "paraformer-realtime":
		return "paraformer-realtime-v2"
	case "paraformer-realtime-v1", "paraformer-realtime_v1":
		return "paraformer-realtime-v1"
	case "fun-asr-realtime", "fun_asr_realtime":
		return "fun-asr-realtime"
	default:
		return raw
	}
}

func dashscopeWSRecognizeWav(ctx context.Context, apiKey string, model string, audioPath string) (string, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return "", errors.New("missing api key")
	}
	model = normalizeDashscopeWSModel(model)
	audioPath = strings.TrimSpace(audioPath)
	if audioPath == "" {
		return "", errors.New("audioPath is required")
	}

	const wsURL = "wss://dashscope.aliyuncs.com/api-ws/v1/inference/"

	header := http.Header{}
	header.Set("Authorization", "bearer "+apiKey)

	dialer := websocket.DefaultDialer
	conn, _, err := dialer.DialContext(ctx, wsURL, header)
	if err != nil {
		zap.L().Error("vectorize_asr_ws_dial_failed",
			zap.String("audio_path", audioPath),
			zap.String("model", model),
			zap.Error(err))
		return "", err
	}
	stopCh := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.SetReadDeadline(time.Now())
			_ = conn.Close()
		case <-stopCh:
		}
	}()
	defer conn.Close()
	defer close(stopCh)

	taskID, err := randomHex(16)
	if err != nil {
		return "", err
	}

	runCmd, err := json.Marshal(map[string]interface{}{
		"header": map[string]interface{}{
			"action":    "run-task",
			"task_id":   taskID,
			"streaming": "duplex",
		},
		"payload": map[string]interface{}{
			"task_group": "audio",
			"task":       "asr",
			"function":   "recognition",
			"model":      model,
			"parameters": map[string]interface{}{
				"format":      "wav",
				"sample_rate": 16000,
			},
			"input": map[string]interface{}{},
		},
	})
	if err != nil {
		return "", err
	}
	if err := conn.WriteMessage(websocket.TextMessage, runCmd); err != nil {
		zap.L().Error("vectorize_asr_ws_run_task_failed",
			zap.String("audio_path", audioPath),
			zap.String("model", model),
			zap.Error(err))
		return "", err
	}

	type wsEvent struct {
		Header struct {
			Event        string `json:"event"`
			TaskID       string `json:"task_id"`
			ErrorCode    string `json:"error_code"`
			ErrorMessage string `json:"error_message"`
		} `json:"header"`
		Payload struct {
			Output struct {
				Sentence struct {
					Text        string `json:"text"`
					SentenceEnd bool   `json:"sentence_end"`
					Heartbeat   bool   `json:"heartbeat"`
				} `json:"sentence"`
			} `json:"output"`
		} `json:"payload"`
	}

	waitStarted := func() error {
		deadline := time.Now().Add(12 * time.Second)
		_ = conn.SetReadDeadline(deadline)
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return err
			}
			var e wsEvent
			if err := json.Unmarshal(msg, &e); err != nil {
				continue
			}
			switch e.Header.Event {
			case "task-started":
				_ = conn.SetReadDeadline(time.Time{})
				return nil
			case "task-failed":
				zap.L().Error("vectorize_asr_ws_task_failed",
					zap.String("audio_path", audioPath),
					zap.String("model", model),
					zap.String("error_code", e.Header.ErrorCode),
					zap.String("error_message", e.Header.ErrorMessage))
				if strings.TrimSpace(e.Header.ErrorMessage) != "" {
					return errors.New(e.Header.ErrorMessage)
				}
				return errors.New("dashscope ws task failed")
			}
		}
	}
	if err := waitStarted(); err != nil {
		zap.L().Error("vectorize_asr_ws_start_failed",
			zap.String("audio_path", audioPath),
			zap.String("model", model),
			zap.Error(err))
		return "", err
	}

	f, err := os.Open(audioPath)
	if err != nil {
		zap.L().Error("vectorize_asr_ws_open_failed",
			zap.String("audio_path", audioPath),
			zap.String("model", model),
			zap.Error(err))
		return "", err
	}
	defer f.Close()

	const bytesPerSecond = 16000 * 1 * 2
	buf := make([]byte, 8*1024)
	streamStart := time.Now()
	var sentBytes int64
	for {
		n, err := f.Read(buf)
		if n > 0 {
			if err := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
				zap.L().Error("vectorize_asr_ws_stream_failed",
					zap.String("audio_path", audioPath),
					zap.String("model", model),
					zap.Error(err))
				return "", err
			}
			sentBytes += int64(n)
			targetElapsed := time.Duration(int64(time.Second) * sentBytes / bytesPerSecond)
			sleepFor := targetElapsed - time.Since(streamStart)
			if sleepFor > 0 {
				select {
				case <-time.After(sleepFor):
				case <-ctx.Done():
					return "", ctx.Err()
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			zap.L().Error("vectorize_asr_ws_read_audio_failed",
				zap.String("audio_path", audioPath),
				zap.String("model", model),
				zap.Error(err))
			return "", err
		}
	}

	finishCmd, err := json.Marshal(map[string]interface{}{
		"header": map[string]interface{}{
			"action":    "finish-task",
			"task_id":   taskID,
			"streaming": "duplex",
		},
		"payload": map[string]interface{}{
			"input": map[string]interface{}{},
		},
	})
	if err != nil {
		return "", err
	}
	if err := conn.WriteMessage(websocket.TextMessage, finishCmd); err != nil {
		zap.L().Error("vectorize_asr_ws_finish_failed",
			zap.String("audio_path", audioPath),
			zap.String("model", model),
			zap.Error(err))
		return "", err
	}

	var parts []string
	var lastNonEmpty string
	doneAt := time.Now().Add(3 * time.Minute)
	for {
		if time.Now().After(doneAt) {
			zap.L().Error("vectorize_asr_ws_timeout",
				zap.String("audio_path", audioPath),
				zap.String("model", model))
			return "", errors.New("dashscope ws timeout")
		}
		_ = conn.SetReadDeadline(time.Now().Add(50 * time.Second))
		_, msg, err := conn.ReadMessage()
		if err != nil {
			zap.L().Error("vectorize_asr_ws_read_failed",
				zap.String("audio_path", audioPath),
				zap.String("model", model),
				zap.Error(err))
			return "", err
		}
		var e wsEvent
		if err := json.Unmarshal(msg, &e); err != nil {
			continue
		}
		switch e.Header.Event {
		case "result-generated":
			if e.Payload.Output.Sentence.Heartbeat {
				continue
			}
			text := strings.TrimSpace(e.Payload.Output.Sentence.Text)
			if text != "" {
				lastNonEmpty = text
			}
			if text != "" && e.Payload.Output.Sentence.SentenceEnd {
				parts = append(parts, text)
			}
		case "task-finished":
			_ = conn.SetReadDeadline(time.Time{})
			if len(parts) > 0 {
				return normalizeText(strings.Join(parts, " ")), nil
			}
			return normalizeText(lastNonEmpty), nil
		case "task-failed":
			zap.L().Error("vectorize_asr_ws_result_failed",
				zap.String("audio_path", audioPath),
				zap.String("model", model),
				zap.String("error_code", e.Header.ErrorCode),
				zap.String("error_message", e.Header.ErrorMessage))
			if strings.TrimSpace(e.Header.ErrorMessage) != "" {
				return "", errors.New(e.Header.ErrorMessage)
			}
			return "", errors.New("dashscope ws task failed")
		}
	}
}

func randomHex(nBytes int) (string, error) {
	if nBytes <= 0 {
		nBytes = 16
	}
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	const hex = "0123456789abcdef"
	out := make([]byte, nBytes*2)
	for i := 0; i < nBytes; i++ {
		out[i*2] = hex[b[i]>>4]
		out[i*2+1] = hex[b[i]&0x0f]
	}
	return string(out), nil
}
