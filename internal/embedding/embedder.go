package embedding

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"

	ort "github.com/yalue/onnxruntime_go"
)

// Embedder loads an ONNX sentence-transformer model and computes 384-dim
// sentence embeddings. The ONNX Runtime global environment is initialized
// once on first use.
type Embedder struct {
	session    *ort.DynamicAdvancedSession
	inputNames []string
}

var (
	ortOnce   sync.Once
	ortInitErr error
)

func ensureORT() error {
	ortOnce.Do(func() {
		// dlopen only searches system paths, never the working directory,
		// so hand it the absolute path of the bundled onnxruntime.so. If the
		// library is absent here, fall back to the loader's default search.
		if abs, err := filepath.Abs("onnxruntime.so"); err == nil {
			if _, statErr := os.Stat(abs); statErr == nil {
				ort.SetSharedLibraryPath(abs)
			}
		}
		ortInitErr = ort.InitializeEnvironment()
	})
	return ortInitErr
}

// NewEmbedder loads the ONNX model at modelPath and returns an Embedder.
func NewEmbedder(modelPath string) (*Embedder, error) {
	if err := ensureORT(); err != nil {
		return nil, fmt.Errorf("onnxruntime init: %w", err)
	}

	if _, err := os.Stat(modelPath); err != nil {
		return nil, fmt.Errorf("model file: %w", err)
	}

	// Discover input/output names from the model.
	inputInfos, outputInfos, err := ort.GetInputOutputInfo(modelPath)
	if err != nil {
		return nil, fmt.Errorf("get model info: %w", err)
	}

	inputNames := make([]string, len(inputInfos))
	for i, info := range inputInfos {
		inputNames[i] = info.Name
	}
	outputNames := make([]string, len(outputInfos))
	for i, info := range outputInfos {
		outputNames[i] = info.Name
	}

	// The ORT shared library path is resolved in ensureORT above.
	session, err := ort.NewDynamicAdvancedSession(
		modelPath, inputNames, outputNames, nil,
	)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	return &Embedder{
		session:    session,
		inputNames: inputNames,
	}, nil
}

// Close releases the underlying ONNX session.
func (e *Embedder) Close() {
	if e.session != nil {
		e.session.Destroy()
		e.session = nil
	}
}

// Embed computes a 384-dim L2-normalised sentence embedding for text.
func (e *Embedder) Embed(text string) ([]float32, error) {
	tokens := tokenize(text)
	if len(tokens) > 128 {
		tokens = tokens[:128]
	}

	seqLen := 128
	inputIDs := make([]int64, seqLen)
	attentionMask := make([]int64, seqLen)
	tokenTypeIDs := make([]int64, seqLen)
	for i, tok := range tokens {
		inputIDs[i] = int64(tok)
		attentionMask[i] = 1
		// tokenTypeIDs stays 0 (single segment)
	}

	shape := ort.NewShape(1, int64(seqLen))

	idTensor, err := ort.NewTensor(shape, inputIDs)
	if err != nil {
		return nil, fmt.Errorf("input_ids tensor: %w", err)
	}
	defer idTensor.Destroy()

	maskTensor, err := ort.NewTensor(shape, attentionMask)
	if err != nil {
		return nil, fmt.Errorf("attention_mask tensor: %w", err)
	}
	defer maskTensor.Destroy()

	typeTensor, err := ort.NewTensor(shape, tokenTypeIDs)
	if err != nil {
		return nil, fmt.Errorf("token_type_ids tensor: %w", err)
	}
	defer typeTensor.Destroy()

	// Build input values in the order the model expects.
	inputs := make([]ort.Value, len(e.inputNames))
	for i, name := range e.inputNames {
		switch name {
		case "input_ids":
			inputs[i] = idTensor
		case "attention_mask":
			inputs[i] = maskTensor
		case "token_type_ids":
			inputs[i] = typeTensor
		default:
			// Fallback: use attention mask for unknown inputs.
			inputs[i] = maskTensor
		}
	}

	// Allocate a single 3-D output: [1, seqLen, 384].
	outShape := ort.NewShape(1, int64(seqLen), 384)
	outTensor, err := ort.NewEmptyTensor[float32](outShape)
	if err != nil {
		return nil, fmt.Errorf("output tensor: %w", err)
	}
	defer outTensor.Destroy()

	outputs := []ort.Value{outTensor}
	if err := e.session.Run(inputs, outputs); err != nil {
		return nil, fmt.Errorf("inference: %w", err)
	}

	// Mean-pool over non-padded positions, then L2-normalise.
	raw := outTensor.GetData()
	embedding := make([]float32, 384)
	var maskSum float32
	for i := 0; i < seqLen; i++ {
		if attentionMask[i] == 1 {
			maskSum++
			base := i * 384
			for j := 0; j < 384; j++ {
				embedding[j] += raw[base+j]
			}
		}
	}
	if maskSum > 0 {
		for j := range embedding {
			embedding[j] /= maskSum
		}
	}

	norm := float32(0)
	for _, v := range embedding {
		norm += v * v
	}
	norm = float32(math.Sqrt(float64(norm)))
	if norm > 0 {
		for i := range embedding {
			embedding[i] /= norm
		}
	}

	return embedding, nil
}

// tokenize is a placeholder tokenizer that splits on whitespace and maps each
// word to a numeric ID via a simple hash. A production implementation would use
// WordPiece with a real vocabulary file matching the ONNX model's expected
// input format.
func tokenize(text string) []int {
	words := splitOnSpaces(text)
	tokens := make([]int, len(words))
	for i, w := range words {
		tokens[i] = simpleHash(w) % 30000
	}
	return tokens
}

func splitOnSpaces(s string) []string {
	var result []string
	current := ""
	for _, c := range s {
		if c == ' ' || c == '\t' || c == '\n' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

func simpleHash(s string) int {
	h := 0
	for _, c := range s {
		h = h*31 + int(c)
	}
	if h < 0 {
		h = -h
	}
	return h
}
