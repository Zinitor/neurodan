"""One-time script to export all-MiniLM-L6-v2 to ONNX format."""
import subprocess
import sys

def main():
    # Install dependencies using uv
    result = subprocess.run(
        [sys.executable, "-m", "pip", "install", "optimum", "onnxruntime", "sentence-transformers"],
        capture_output=True, text=True
    )
    if result.returncode != 0:
        print(f"pip install failed: {result.stderr}")
        return 1

    from optimum.onnxruntime import ORTModelForFeatureExtraction
    from transformers import AutoTokenizer

    model_id = "sentence-transformers/all-MiniLM-L6-v2"
    tokenizer = AutoTokenizer.from_pretrained(model_id)
    model = ORTModelForFeatureExtraction.from_pretrained(model_id, export=True)

    tokenizer.save_pretrained("./models")
    model.save_pretrained("./models")
    print("Model exported successfully to ./models/")
    return 0

if __name__ == "__main__":
    sys.exit(main())