package videoapp

import runtimeapp "nlp-video-analysis/internal/application/videoapp/runtime"

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
