# Two Tower User Feature Training Design

## Goal

Make the two-tower recommender better match this education product by adding user learning features to the user tower, while keeping the existing sample CSV, embedding artifact, importer, and publish gate contracts compatible.

## Current State

The current training loop uses behavior samples from video reaction, segment reaction, watch records, and recommendation exposure. The item tower already has a feature MLP fed by item catalog fields such as segment duration, video duration, summary, knowledge tags, title, and reaction counts. The user tower is still mostly `user_id` embedding, so it cannot directly use grade, class, knowledge mastery, answer correctness, question search, or generated-question feedback.

Remote database inspection shows several useful education tables:

- `sys_user`: `grade_id`, `class_id`, `user_type`.
- `edu_user_knowledge_mastery`: mastery, correct count, incorrect count per knowledge point.
- `edu_knowledge_answer_record`: correctness, score rate, cost seconds, question and knowledge point history.
- `edu_user_question_feedback`: mastery feedback count per question.
- `edu_question_search_record`: recent search and question knowledge text.
- `edu_generated_question_feedback`: generated-question feedback, correctness, and score rate.

## Design

Add a new optional CSV input named `two_tower_user_features.csv`.

The Go exporter will add `--user-output`. When set, it writes one row per valid `sys_user` with numeric aggregates and text tokens:

- Identity/context: `user_id`, `grade_id`, `class_id`, `user_type`.
- Knowledge mastery: average mastery, minimum mastery, weak/strong knowledge counts, correct/incorrect totals.
- Answer behavior: answer count, correct/incorrect counts, average score rate, average cost seconds, recent knowledge point IDs, recent subjects.
- Feedback/search behavior: feedback count, generated question feedback count, generated average score rate, search count, search knowledge text.

The Python trainer will add `--user-features`. When supplied, `train.py` loads the user feature CSV into `Dataset.user_features`. The PyTorch user tower becomes:

```text
normalize(user_id_embedding + user_feature_mlp(user_feature_vector))
```

If a user has no feature row, the feature vector is all zero. The existing SGD backend remains compatible and ignores user features. Existing artifacts stay unchanged: `user_embeddings.csv`, `item_embeddings.csv`, maps, and `metrics.json` keep their current shape. This means `import_two_tower_embeddings` does not need to change.

The pipeline will write:

```text
two-tower-training/data/${MODEL_VERSION}_user_features.csv
```

and pass it to `python3 -m two_tower_training.train --user-features`.

## Scheduling

Change the two-tower trainer schedule from the current three daily runs to every four hours from Beijing time `00:00`:

```text
00:00, 04:00, 08:00, 12:00, 16:00, 20:00
```

The scheduler still prevents overlapping runs.

## Safety

The user-feature export should be resilient to partially missing optional education tables by emitting neutral values for missing aggregate sources. The current production database contains the intended tables, but local or older environments should not fail just because a learning-behavior table is absent.

Publish gate behavior stays unchanged. If the new user-feature model performs worse than the previous active model beyond configured thresholds, the artifact is retained and not published.

## Verification

- Go unit tests for `export_two_tower_samples` parsing, user CSV writing, and query/table fallback behavior.
- Python unit tests for loading user features and using them in the PyTorch user tower.
- Pipeline test updated to expect `--user-output` and cleanup of generated user feature CSV.
- Scheduler tests updated for the every-four-hours schedule.
- Existing two-tower Python and Go tool tests must pass.
