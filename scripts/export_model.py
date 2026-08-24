"""One-time script to export all-MiniLM-L6-v2 to ONNX format."""
import os
import sys

import torch
from transformers import AutoModel, AutoTokenizer

MODEL_ID = "sentence-transformers/all-MiniLM-L6-v2"
OUTPUT_DIR = "./models"


def main():
    tokenizer = AutoTokenizer.from_pretrained(MODEL_ID)
    model = AutoModel.from_pretrained(MODEL_ID)
    model.eval()

    class Wrapper(torch.nn.Module):
        def __init__(self):
            super().__init__()
            self.model = model

        def forward(self, input_ids, attention_mask, token_type_ids):
            out = self.model(
                input_ids=input_ids,
                attention_mask=attention_mask,
                token_type_ids=token_type_ids,
            )
            return out.last_hidden_state

    seq_len = 128
    batch = 1
    dummy = (
        torch.ones(batch, seq_len, dtype=torch.int64),
        torch.ones(batch, seq_len, dtype=torch.int64),
        torch.zeros(batch, seq_len, dtype=torch.int64),
    )
    os.makedirs(OUTPUT_DIR, exist_ok=True)
    torch.onnx.export(
        Wrapper(),
        dummy,
        os.path.join(OUTPUT_DIR, "model.onnx"),
        input_names=["input_ids", "attention_mask", "token_type_ids"],
        output_names=["last_hidden_state"],
        dynamic_axes={
            "input_ids": {0: "batch_size"},
            "attention_mask": {0: "batch_size"},
            "token_type_ids": {0: "batch_size"},
            "last_hidden_state": {0: "batch_size"},
        },
        opset_version=14,
        dynamo=False,
    )
    tokenizer.save_pretrained(OUTPUT_DIR)
    print(f"Model exported successfully to {OUTPUT_DIR}/model.onnx")
    return 0


if __name__ == "__main__":
    sys.exit(main())
