package mock

import (
	"sync"
	"time"

	rsmetering "github.com/pingcap/kvproto/pkg/resource_usage_agent"
	"github.com/pingcap/ng-monitoring/component/topsql/store"
	"github.com/pingcap/tipb/go-tipb"
)

type MemStore struct {
	sync.Mutex

	// instance -> value
	Instances map[string]struct {
		Instance     string
		InstanceType string
	}

	// instance -> sql digest -> plan digest -> records
	TopSQLRecords map[string]map[string]map[string]*tipb.CPUTimeRecord

	// instance -> resource tag -> records
	ResourceMeteringRecords map[string]map[string]*rsmetering.ResourceUsageRecord

	// SQL digest -> meta
	SQLMetas map[string]struct {
		Meta *tipb.SQLMeta
	}

	// plan digest -> meta
	PlanMetas map[string]struct {
		Meta *tipb.PlanMeta
	}
}

func NewMemStore() *MemStore {
	return &MemStore{
		Instances: make(map[string]struct {
			Instance     string
			InstanceType string
		}),
		TopSQLRecords:           make(map[string]map[string]map[string]*tipb.CPUTimeRecord),
		ResourceMeteringRecords: make(map[string]map[string]*rsmetering.ResourceUsageRecord),
		SQLMetas: make(map[string]struct {
			Meta *tipb.SQLMeta
		}),
		PlanMetas: make(map[string]struct {
			Meta *tipb.PlanMeta
		}),
	}
}

func (m *MemStore) Predict(pred func(*MemStore) bool, beginWaitTime time.Duration, maxWaitTime time.Duration) bool {
	begin := time.Now()
	timeToWait := beginWaitTime

	for {
		passed := func() bool {
			m.Lock()
			defer m.Unlock()

			return pred(m)
		}()

		waitedTime := time.Since(begin)
		if passed {
			return true
		} else if waitedTime >= maxWaitTime {
			return false
		}

		if waitedTime+timeToWait > maxWaitTime {
			timeToWait = maxWaitTime - waitedTime
		}
		time.Sleep(timeToWait)
		timeToWait *= 2
	}
}

var _ store.Store = &MemStore{}

func (m *MemStore) Instance(instance, instanceType string) error {
	m.Lock()
	m.Instances[instance] = struct {
		Instance     string
		InstanceType string
	}{Instance: instance, InstanceType: instanceType}
	m.Unlock()

	return nil
}

func (m *MemStore) TopSQLRecord(instance, _ string, record *tipb.CPUTimeRecord) error {
	m.Lock()
	if _, ok := m.TopSQLRecords[instance]; !ok {
		m.TopSQLRecords[instance] = make(map[string]map[string]*tipb.CPUTimeRecord)
	}
	if _, ok := m.TopSQLRecords[instance][string(record.SqlDigest)]; !ok {
		m.TopSQLRecords[instance][string(record.SqlDigest)] = make(map[string]*tipb.CPUTimeRecord)
	}
	if _, ok := m.TopSQLRecords[instance][string(record.SqlDigest)][string(record.PlanDigest)]; !ok {
		m.TopSQLRecords[instance][string(record.SqlDigest)][string(record.PlanDigest)] = &tipb.CPUTimeRecord{
			SqlDigest:  record.SqlDigest,
			PlanDigest: record.PlanDigest,
		}
	}
	r := m.TopSQLRecords[instance][string(record.SqlDigest)][string(record.PlanDigest)]
	r.RecordListTimestampSec = append(r.RecordListTimestampSec, record.RecordListTimestampSec...)
	r.RecordListCpuTimeMs = append(r.RecordListCpuTimeMs, record.RecordListCpuTimeMs...)
	m.Unlock()

	return nil
}

func (m *MemStore) ResourceMeteringRecord(instance, _ string, record *rsmetering.ResourceUsageRecord) error {
	m.Lock()
	if _, ok := m.ResourceMeteringRecords[instance]; !ok {
		m.ResourceMeteringRecords[instance] = make(map[string]*rsmetering.ResourceUsageRecord)
	}
	if _, ok := m.ResourceMeteringRecords[instance][string(record.ResourceGroupTag)]; !ok {
		m.ResourceMeteringRecords[instance][string(record.ResourceGroupTag)] = &rsmetering.ResourceUsageRecord{
			ResourceGroupTag: record.ResourceGroupTag,
		}
	}
	r := m.ResourceMeteringRecords[instance][string(record.ResourceGroupTag)]
	r.RecordListTimestampSec = append(r.RecordListTimestampSec, record.RecordListTimestampSec...)
	r.RecordListCpuTimeMs = append(r.RecordListCpuTimeMs, record.RecordListCpuTimeMs...)
	r.RecordListReadKeys = append(r.RecordListReadKeys, record.RecordListReadKeys...)
	r.RecordListWriteKeys = append(r.RecordListWriteKeys, record.RecordListWriteKeys...)
	m.Unlock()

	return nil
}

func (m *MemStore) SQLMeta(meta *tipb.SQLMeta) error {
	m.Lock()
	m.SQLMetas[string(meta.SqlDigest)] = struct{ Meta *tipb.SQLMeta }{Meta: meta}
	m.Unlock()

	return nil
}

func (m *MemStore) PlanMeta(meta *tipb.PlanMeta) error {
	m.Lock()
	m.PlanMetas[string(meta.PlanDigest)] = struct{ Meta *tipb.PlanMeta }{Meta: meta}
	m.Unlock()

	return nil
}

func (m *MemStore) Close() {
}
