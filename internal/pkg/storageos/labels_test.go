package storageos

import "testing"

func TestIsReservedLabel(t *testing.T) {
	testcases := []struct {
		key          string
		wantReserved bool
	}{
		{
			key:          "not-storage-os",
			wantReserved: false,
		},
		{
			key:          ReservedLabelPrefix,
			wantReserved: true,
		},
		{
			key:          ReservedLabelPrefix + "foo",
			wantReserved: true,
		},
		{
			key:          ReservedLabelK8sPVCNamespace,
			wantReserved: true,
		},
		{
			key:          ReservedLabelK8sPVCName,
			wantReserved: true,
		},
		{
			key:          ReservedLabelK8sPVName,
			wantReserved: true,
		},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.key, func(t *testing.T) {
			reserved := IsReservedLabel(tc.key)

			if reserved != tc.wantReserved {
				t.Errorf("reserved must match got:\n%t\n, want:\n%t", reserved, tc.wantReserved)
			}
		})
	}
}
