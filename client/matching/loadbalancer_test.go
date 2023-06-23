package matching

import (
	"github.com/stretchr/testify/assert"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/types"
	"testing"
)

func Test_defaultLoadBalancer_PickWritePartition(t *testing.T) {
	type fields struct {
		nReadPartitions  dynamicconfig.IntPropertyFnWithTaskListInfoFilters
		nWritePartitions dynamicconfig.IntPropertyFnWithTaskListInfoFilters
		domainIDToName   func(string) (string, error)
	}
	type args struct {
		domainID      string
		taskList      types.TaskList
		taskListType  int
		forwardedFrom string
	}
	tests := []struct {
		name        string
		fields      fields
		args        args
		expectedRes []string
	}{
		{
			name: "in dynamic config, write larger than read",
			fields: fields{
				domainIDToName: func(s string) (string, error) {
					return s, nil
				},
				nReadPartitions: func(domain string, taskList string, taskType int) int {
					return 1
				},
				nWritePartitions: func(domain string, taskList string, taskType int) int {
					return 20
				},
			},
			expectedRes: []string{},
		},
		{
			name: "in dynamic config, write equals read",
		},
		{
			name: "in dynamic config, write equals read",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lb := &defaultLoadBalancer{
				nReadPartitions:  tt.fields.nReadPartitions,
				nWritePartitions: tt.fields.nWritePartitions,
				domainIDToName:   tt.fields.domainIDToName,
			}
			for i := 0; i < 100; i++ {
				tl := lb.PickWritePartition(tt.args.domainID, tt.args.taskList, tt.args.taskListType, tt.args.forwardedFrom)
				assert.Contains(t, tt.expectedRes, tl)
			}
		})
	}
}
