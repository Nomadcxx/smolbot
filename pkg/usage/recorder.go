package usage

import "context"

type Recorder interface {
	RecordCompletion(ctx context.Context, record CompletionRecord) error
}
