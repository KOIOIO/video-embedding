package videoapp

import runtimeapp "video-service/internal/application/videoapp/runtime"

type ActiveCounterStore = runtimeapp.ActiveCounterStore

var runtimeCounters ActiveCounterStore = runtimeapp.NewMemoryActiveCounterStore()

func RuntimeCounters() ActiveCounterStore {
	return runtimeCounters
}

func SetRuntimeCounters(store ActiveCounterStore) {
	if store == nil {
		runtimeCounters = runtimeapp.NewMemoryActiveCounterStore()
		return
	}
	runtimeCounters = store
}
