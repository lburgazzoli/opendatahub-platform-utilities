//nolint:exhaustruct_v5 // Sparse literals keep focused builder fixtures readable.
package reconciler

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestHasDynamicWatches(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		watches []watchRegistration
		want    bool
	}{
		"no watches": {},
		"static watch": {
			watches: []watchRegistration{{options: WatchOptions{Dynamic: false}}},
		},
		"dynamic watch": {
			watches: []watchRegistration{{options: WatchOptions{Dynamic: true}}},
			want:    true,
		},
		"mixed watches": {
			watches: []watchRegistration{
				{options: WatchOptions{Dynamic: false}},
				{options: WatchOptions{Dynamic: true}},
			},
			want: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			g := NewWithT(t)
			builder := &Builder{watch: test.watches}
			g.Expect(builder.hasDynamicWatches()).To(Equal(test.want))
		})
	}
}
