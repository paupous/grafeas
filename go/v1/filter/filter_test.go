package filter_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/grafeas/grafeas/go/v1/filter"
	gpb "github.com/grafeas/grafeas/proto/v1/grafeas_go_proto"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakeLogger struct{}

func (fakeLogger) PrepareCtx(ctx context.Context, projectID string) context.Context {
	return ctx
}
func (fakeLogger) Info(ctx context.Context, args ...interface{})                    {}
func (fakeLogger) Infof(ctx context.Context, format string, args ...interface{})    {}
func (fakeLogger) Warning(ctx context.Context, args ...interface{})                 {}
func (fakeLogger) Warningf(ctx context.Context, format string, args ...interface{}) {}
func (fakeLogger) Error(ctx context.Context, args ...interface{})                   {}
func (fakeLogger) Errorf(ctx context.Context, format string, args ...interface{})   {}

func TestListOccsFilter(t *testing.T) {
	byResourceUri := func(ctx context.Context, projID string, filter, pageToken string, pageSize int32) ([]*gpb.Occurrence, string, bool, error) {
		if strings.HasPrefix(filter, "resourceUri = ") {
			return []*gpb.Occurrence{{Name: "1234-abcd"}}, "", true, nil
		}
		return nil, "", false, nil
	}
	defaultFilterFn := func(ctx context.Context, projID string, filter, pageToken string, pageSize int32) ([]*gpb.Occurrence, string, bool, error) {
		return []*gpb.Occurrence{{Name: "7777-8888"}}, "", true, nil
	}

	f := filter.ListOccsFilterer{
		Log:             &fakeLogger{},
		FilterFns:       []filter.ListOccsFilterFn{byResourceUri},
		DefaultFilterFn: defaultFilterFn,
	}

	tests := []struct {
		desc     string
		filter   string
		wantOccs []*gpb.Occurrence
	}{
		{
			desc:   "no filter functions match, default filter function is used",
			filter: `updateTime = "2019-01-01"`,
			wantOccs: []*gpb.Occurrence{
				{
					Name: "7777-8888",
				},
			},
		},
		{
			desc:   "a filter function match",
			filter: `resourceUri = "foobar"`,
			wantOccs: []*gpb.Occurrence{
				{
					Name: "1234-abcd",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			ctx := context.Background()

			occs, _, err := f.Filter(ctx, "my-proj", tt.filter, "", 0)
			if err != nil {
				t.Fatalf(`Filter("my-proj", %q, "", 0) failed with: %v`, tt.filter, err)
			}

			if diff := cmp.Diff(tt.wantOccs, occs); diff != "" {
				t.Errorf("Filter(\"my-proj\", %q, \"\", 0) returned diff -want +got\n%s", tt.filter, diff)
			}
		})
	}
}

func TestListOccsFilterErrors(t *testing.T) {
	byResourceUri := func(ctx context.Context, projID string, filter, pageToken string, pageSize int32) ([]*gpb.Occurrence, string, bool, error) {
		if strings.HasPrefix(filter, "resourceUri = ") {
			return nil, "", true, status.Errorf(codes.Internal, "error executing filter")
		}
		return nil, "", false, nil
	}
	defaultFilterFn := func(ctx context.Context, projID string, filter, pageToken string, pageSize int32) ([]*gpb.Occurrence, string, bool, error) {
		return nil, "", true, status.Errorf(codes.InvalidArgument, "argument not valid")
	}

	f := filter.ListOccsFilterer{
		Log:             &fakeLogger{},
		FilterFns:       []filter.ListOccsFilterFn{byResourceUri},
		DefaultFilterFn: defaultFilterFn,
	}

	tests := []struct {
		desc        string
		filter      string
		wantErrCode codes.Code
	}{
		{
			desc:        "no filter functions match, default filter function is used, but there is an error",
			filter:      `updateTime = "2019-01-01"`,
			wantErrCode: codes.InvalidArgument,
		},
		{
			desc:        "a filter function match, but there is an error",
			filter:      `resourceUri = "foobar"`,
			wantErrCode: codes.Internal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			ctx := context.Background()

			_, _, err := f.Filter(ctx, "my-proj", tt.filter, "", 0)
			if c := status.Code(err); c != tt.wantErrCode {
				t.Errorf(`Filter("my-proj", %q, "", 0) got error code %v, want %v`, tt.filter, c, tt.wantErrCode)
			}
		})
	}
}