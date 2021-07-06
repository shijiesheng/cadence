// Copyright (c) 2020 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination interface_mock.go -self_package github.com/uber/cadence/service/history/task

package task

import (
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/future"
	"github.com/uber/cadence/common/task"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/shard"
)

type (
	// Info contains the metadata for a task
	Info interface {
		GetVersion() int64
		GetTaskID() int64
		GetTaskType() int
		GetVisibilityTimestamp() time.Time
		GetWorkflowID() string
		GetRunID() string
		GetDomainID() string
	}

	// Task is the interface for all tasks generated by history service
	Task interface {
		task.PriorityTask
		Info
		GetQueueType() QueueType
		GetShard() shard.Context
		GetAttempt() int
	}

	CrossClusterTask interface {
		Task
		IsReadyForPoll() bool
		IsValid() bool
		Update(interface{}) error //TODO: update interface once the cross cluster response idl lands
		GetCrossClusterRequest() *types.CrossClusterTaskRequest
	}

	// Key identifies a Task and defines a total order among tasks
	Key interface {
		Less(Key) bool
	}

	// Executor contains the execution logic for Task
	Executor interface {
		Execute(taskInfo Info, shouldProcessTask bool) error
	}

	// Filter filters Task
	Filter func(task Info) (bool, error)

	// Initializer initializes a Task based on the Info
	Initializer func(Info) Task

	// PriorityAssigner assigns priority to Tasks
	PriorityAssigner interface {
		Assign(Task) error
	}

	// Processor is the worker pool for processing Tasks
	Processor interface {
		common.Daemon
		StopShardProcessor(shard.Context)
		Submit(Task) error
		TrySubmit(Task) (bool, error)
	}

	// Redispatcher buffers tasks and periodically redispatch them to Processor
	// redispatch can also be triggered immediately by calling the Redispatch method
	Redispatcher interface {
		common.Daemon
		AddTask(Task)
		Redispatch(targetSize int)
		Size() int
	}

	// Fetcher is a host level component for aggregating task fetch requests
	// from all shards on the host and perform one fetching operation for
	// aggregated requests.
	Fetcher interface {
		common.Daemon
		GetSourceCluster() string
		Fetch(shardID int, fetchParams ...interface{}) future.Future
	}

	//Fetchers is a group of Fetchers, one for each source cluster
	Fetchers []Fetcher

	// QueueType is the type of task queue
	QueueType int
)

const (
	// QueueTypeActiveTransfer is the queue type for active transfer queue processor
	QueueTypeActiveTransfer QueueType = iota + 1
	// QueueTypeStandbyTransfer is the queue type for standby transfer queue processor
	QueueTypeStandbyTransfer
	// QueueTypeActiveTimer is the queue type for active timer queue processor
	QueueTypeActiveTimer
	// QueueTypeStandbyTimer is the queue type for standby timer queue processor
	QueueTypeStandbyTimer
	// QueueTypeReplication is the queue type for replication queue processor
	QueueTypeReplication
	// QueueTypeCrossCluster is the queue type for cross cluster queue processor
	QueueTypeCrossCluster
)
