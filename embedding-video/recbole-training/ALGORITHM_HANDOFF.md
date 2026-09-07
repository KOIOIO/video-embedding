# RecBole Algorithm Handoff

The current production candidate is RecBole `BPR` with `embedding_size=64`. It preserves dot-product style user/item embeddings, so online retrieval can remain pgvector-based. The checked-in service configs select `Recommendation.Engine=recbole`; Gorse remains an optional external candidate integration.

Expected artifact contract:

```text
item_embeddings.csv: video_segment_id,video_id,embedding,model_version
user_embeddings.csv: user_id,embedding,model_version
metrics.json: framework,algorithm,model_version,Recall@20,NDCG@20,Hit@20,Precision@20
```

The data source is broader than video-only behavior:

- `.inter` contains explicit video interactions, watch/exposure events, and question-search video follow-up events.
- `.user` contains user profile, mastery, answering, feedback, search, special-practice, vocabulary, English reading/listening/storybook, and profile snapshot features.
- `.item` contains video segment and content metadata.

Publication contract: the pipeline writes the artifact files, evaluates the RecBole metrics, compares them with the active version, and only then imports `recsys` embeddings and publishes the model version. Gate failure must retain the previous active version.
