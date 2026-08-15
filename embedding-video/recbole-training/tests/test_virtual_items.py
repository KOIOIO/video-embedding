import unittest

from recbole_recommendation.virtual_items import install_full_sort_mask, is_virtual_token, mask_virtual_item_scores


class VirtualItemsTest(unittest.TestCase):
    def test_masks_virtual_scores(self):
        scores = [[0.1, 0.9, 0.8]]
        self.assertEqual(mask_virtual_item_scores(scores, [1]), [[0.1, float("-inf"), 0.8]])
        self.assertTrue(is_virtual_token("knowledge_video:88"))

    def test_installs_mask_at_model_full_sort_boundary(self):
        class Dataset:
            iid_field = "item_id"
            field2id_token = {"item_id": ["[PAD]", "101", "knowledge_video:88"]}

        class Model:
            def full_sort_predict(self, _interaction):
                return [[0.0, 0.2, 0.9]]

        model = Model()
        self.assertEqual(install_full_sort_mask(model, Dataset()), [2])
        self.assertEqual(model.full_sort_predict(None), [[0.0, 0.2, float("-inf")]])

    def test_masks_flattened_tensor_scores(self):
        class FakeTensor:
            def __init__(self, values, shape=None):
                self.values = values
                self.shape = shape or (len(values),)
            def reshape(self, *shape):
                if shape == (-1, 3): return FakeMatrix([self.values[:3], self.values[3:]], self.shape)
                return self
        class FakeMatrix:
            def __init__(self, rows, original_shape): self.rows, self.original_shape = rows, original_shape
            def clone(self): return FakeMatrix([row[:] for row in self.rows], self.original_shape)
            def __setitem__(self, key, value):
                for row in self.rows: row[key[1][0]] = value
            def reshape(self, _shape): return [value for row in self.rows for value in row]
        class Dataset:
            iid_field = "item_id"
            field2id_token = {"item_id": ["[PAD]", "101", "knowledge_video:88"]}
        class Model:
            def full_sort_predict(self, _interaction): return FakeTensor([0, 0.2, 0.9, 0, 0.3, 0.8])
        model = Model()
        install_full_sort_mask(model, Dataset())
        self.assertEqual(model.full_sort_predict(None), [0, 0.2, float("-inf"), 0, 0.3, float("-inf")])


if __name__ == "__main__":
    unittest.main()
