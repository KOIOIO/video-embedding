from __future__ import annotations


VIRTUAL_ITEM_PREFIX = "knowledge_video:"


def is_virtual_token(token: str) -> bool:
    return str(token).startswith(VIRTUAL_ITEM_PREFIX)


def mask_virtual_item_scores(scores, internal_ids: list[int]):
    if hasattr(scores, "clone"):
        result = scores.clone()
        if internal_ids:
            result[..., internal_ids] = float("-inf")
        return result
    result = [list(row) for row in scores]
    for row in result:
        for internal_id in internal_ids:
            row[internal_id] = float("-inf")
    return result


def virtual_internal_ids(dataset) -> list[int]:
    tokens = dataset.field2id_token[dataset.iid_field]
    return [index for index, token in enumerate(tokens) if is_virtual_token(str(token))]


def install_full_sort_mask(model, dataset):
    internal_ids = virtual_internal_ids(dataset)
    item_count = len(dataset.field2id_token[dataset.iid_field])
    original = model.full_sort_predict

    def masked_full_sort_predict(interaction):
        scores = original(interaction)
        if hasattr(scores, "reshape"):
            original_shape = scores.shape
            masked = mask_virtual_item_scores(scores.reshape(-1, item_count), internal_ids)
            return masked.reshape(original_shape)
        return mask_virtual_item_scores(scores, internal_ids)

    model.full_sort_predict = masked_full_sort_predict
    return internal_ids
