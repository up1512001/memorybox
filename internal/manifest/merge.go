package manifest

import (
	"context"
	"io"
)

// MergeEvent is emitted for each path seen in either manifest.
// Left is nil when the path exists only in the right manifest (added).
// Right is nil when the path exists only in the left manifest (deleted).
// Both are non-nil when the path exists in both (possibly modified).
type MergeEvent struct {
	Left  *Entry
	Right *Entry
}

// Merge performs an O(n) sort-merge of two sorted Readers, emitting one
// MergeEvent per unique path into out. Both readers must be in ascending
// lexicographic path order — Writer always writes in this order.
//
// Merge closes out when finished or when ctx is cancelled.
func Merge(ctx context.Context, left, right *Reader, out chan<- MergeEvent) error {
	defer close(out)

	lEntry, lErr := nextOrNil(left)
	rEntry, rErr := nextOrNil(right)

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Both exhausted.
		if lEntry == nil && rEntry == nil {
			if lErr != nil && lErr != io.EOF {
				return lErr
			}
			if rErr != nil && rErr != io.EOF {
				return rErr
			}
			return nil
		}

		// Left exhausted — drain right.
		if lEntry == nil {
			out <- MergeEvent{Right: rEntry}
			rEntry, rErr = nextOrNil(right)
			continue
		}

		// Right exhausted — drain left.
		if rEntry == nil {
			out <- MergeEvent{Left: lEntry}
			lEntry, lErr = nextOrNil(left)
			continue
		}

		switch {
		case lEntry.Path < rEntry.Path:
			out <- MergeEvent{Left: lEntry}
			lEntry, lErr = nextOrNil(left)
		case lEntry.Path > rEntry.Path:
			out <- MergeEvent{Right: rEntry}
			rEntry, rErr = nextOrNil(right)
		default:
			out <- MergeEvent{Left: lEntry, Right: rEntry}
			lEntry, lErr = nextOrNil(left)
			rEntry, rErr = nextOrNil(right)
		}

		if lErr != nil && lErr != io.EOF {
			return lErr
		}
		if rErr != nil && rErr != io.EOF {
			return rErr
		}
	}
}

func nextOrNil(r *Reader) (*Entry, error) {
	e, err := r.Next()
	if err == io.EOF {
		return nil, io.EOF
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}
