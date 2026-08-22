// Package markov implements a variable-order Markov chain text generator.
//
// Unlike a fixed-order chain, every training message emits transitions of
// order 1, 2 and 3. During generation the order is re-rolled on every step
// from a per-preset probability distribution, so the same model can produce
// short, erratic bursts or long, coherent stretches depending on the
// requested length preset.
package markov

import (
	"math/rand"
	"strings"
	"sync"
	"unicode/utf8"
)

const maxOrder = 3

// Transition is a single n-gram step: given the Prefix (of length Order),
// NextWord follows. Weight is the aggregated count across all training
// messages.
type Transition struct {
	Order    int
	Prefix   []string
	NextWord string
	Weight   int
}

// LengthPreset selects how long (and therefore how coherent) generated text
// should be. Each preset is a probability distribution over n-gram orders.
type LengthPreset int

const (
	LengthShort LengthPreset = iota
	LengthMedium
	LengthLong
)

// ParseLengthPreset parses a preset name ("short", "medium", "long").
func ParseLengthPreset(s string) (LengthPreset, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "short":
		return LengthShort, true
	case "medium":
		return LengthMedium, true
	case "long":
		return LengthLong, true
	default:
		return LengthMedium, false
	}
}

func (p LengthPreset) String() string {
	switch p {
	case LengthShort:
		return "short"
	case LengthLong:
		return "long"
	default:
		return "medium"
	}
}

// lengthConfig holds the parameters that define a length preset.
type lengthConfig struct {
	minWords   int
	maxWords   int
	orderProbs [maxOrder]float64 // P(order=1), P(order=2), P(order=3)
}

var lengthConfigs = map[LengthPreset]lengthConfig{
	// short: 1-5 words, heavily order-1 so output is erratic and punchy.
	LengthShort: {minWords: 1, maxWords: 5, orderProbs: [3]float64{0.7, 0.25, 0.05}},
	// medium: 6-15 words, balanced.
	LengthMedium: {minWords: 6, maxWords: 15, orderProbs: [3]float64{0.15, 0.45, 0.4}},
	// long: 16-40 words, prefers order-3 so text is more grammatical.
	LengthLong: {minWords: 16, maxWords: 40, orderProbs: [3]float64{0.05, 0.35, 0.6}},
}

// GenerateOptions configures a single generation pass.
type GenerateOptions struct {
	// Seed, when non-empty, forces the generated text to start with these
	// words. Takes precedence over Context.
	Seed string
	// Context is recent conversation text. When Seed is empty, the longest
	// suffix of Context that appears as a prefix in the model is used to seed
	// generation, letting the output pick up the conversation.
	Context string
	// Length selects the length preset. Defaults to LengthMedium.
	Length LengthPreset
}

// candidate is a possible next word with an aggregate weight.
type candidate struct {
	next   string
	weight int
}

// index is an in-memory lookup of transitions keyed by order and prefix.
type index struct {
	// m[order][prefixKey][nextWord] = aggregateWeight
	m map[int]map[string]map[string]int
}

func newIndex(transitions []Transition) *index {
	idx := &index{m: make(map[int]map[string]map[string]int, maxOrder)}
	for _, t := range transitions {
		order := t.Order
		if order < 1 || order > maxOrder || len(t.Prefix) != order {
			order = len(t.Prefix)
		}
		if order < 1 || order > maxOrder {
			continue
		}
		if idx.m[order] == nil {
			idx.m[order] = make(map[string]map[string]int)
		}
		key := strings.Join(t.Prefix, "\x00")
		if idx.m[order][key] == nil {
			idx.m[order][key] = make(map[string]int)
		}
		idx.m[order][key][t.NextWord] += t.Weight
	}
	return idx
}

// hasPrefix reports whether words (length 1..maxOrder) is a known prefix.
func (idx *index) hasPrefix(words []string) bool {
	order := len(words)
	if order < 1 || order > maxOrder {
		return false
	}
	key := strings.Join(words, "\x00")
	return len(idx.m[order][key]) > 0
}

// randomStart returns a random order-1 prefix to seed generation from when no
// seed or context is available. It returns nil if the model has no order-1
// transitions.
func (idx *index) randomStart(r *rand.Rand) []string {
	order1 := idx.m[1]
	if len(order1) == 0 {
		return nil
	}
	i := r.Intn(len(order1))
	for prefix := range order1 {
		if i == 0 {
			return strings.Split(prefix, "\x00")
		}
		i--
	}
	return nil
}

// next samples the next word following the given context words, attempting
// the requested order first and falling back to lower orders. ok is false if
// no continuation exists.
func (idx *index) next(r *rand.Rand, words []string, probs *[maxOrder]float64) (string, bool) {
	start := pickOrder(r, probs)
	for order := start; order >= 1; order-- {
		if len(words) < order {
			continue
		}
		key := strings.Join(words[len(words)-order:], "\x00")
		cands := idx.m[order][key]
		if len(cands) == 0 {
			continue
		}
		return weightedPick(r, cands)
	}
	return "", false
}

// pickOrder samples an n-gram order from the preset's probability
// distribution. Higher orders mean longer, more coherent context.
func pickOrder(r *rand.Rand, probs *[maxOrder]float64) int {
	roll := r.Float64()
	for i := 0; i < maxOrder; i++ {
		if roll < probs[i] {
			return i + 1
		}
		roll -= probs[i]
	}
	return 1
}

func weightedPick(r *rand.Rand, cands map[string]int) (string, bool) {
	total := 0
	for _, w := range cands {
		total += w
	}
	if total <= 0 {
		return "", false
	}
	roll := r.Intn(total)
	for word, w := range cands {
		roll -= w
		if roll < 0 {
			return word, true
		}
	}
	return "", false
}

// Engine is safe for concurrent use.
type Engine struct {
	mu  sync.Mutex
	rng *rand.Rand
}

func NewEngine() *Engine {
	return &Engine{rng: rand.New(rand.NewSource(rand.Int63()))}
}

// Train extracts variable-order (1..3) transitions from text. Each transition
// has weight 1; weights are aggregated when persisted.
func (e *Engine) Train(text string) []Transition {
	words := strings.Fields(text)
	if len(words) < 2 {
		return nil
	}

	var transitions []Transition
	for i := 0; i < len(words); i++ {
		for order := 1; order <= maxOrder; order++ {
			nextIdx := i + order
			if nextIdx >= len(words) {
				continue
			}
			prefix := make([]string, order)
			copy(prefix, words[i:nextIdx])
			transitions = append(transitions, Transition{
				Order:    order,
				Prefix:   prefix,
				NextWord: words[nextIdx],
				Weight:   1,
			})
		}
	}
	return transitions
}

// Generate produces a random walk through the transition graph according to
// opts. Generation starts from opts.Seed, or — when Seed is empty — from the
// keyword bridge over opts.Context, or — when neither is available — from a
// random order-1 prefix. It then continues for as many words as the length
// preset asks for, or until no continuation exists.
func (e *Engine) Generate(transitions []Transition, opts GenerateOptions) string {
	e.mu.Lock()
	defer e.mu.Unlock()

	idx := newIndex(transitions)
	if len(idx.m) == 0 {
		return ""
	}

	var result []string
	seed := opts.Seed
	if seed == "" && opts.Context != "" {
		seed = bridge(idx, opts.Context)
	}
	if seed != "" {
		result = append(result, strings.Fields(seed)...)
	} else if start := idx.randomStart(e.rng); start != nil {
		// No seed and no usable context: start from a random prefix so the
		// bot can still self-initiate.
		result = append(result, start...)
	}

	cfg, ok := lengthConfigs[opts.Length]
	if !ok {
		cfg = lengthConfigs[LengthMedium]
	}
	target := cfg.minWords + e.rng.Intn(cfg.maxWords-cfg.minWords+1)

	// Bound iterations so a degenerate order-1 graph cannot loop forever.
	for i := 0; len(result) < target && i < target*4; i++ {
		next, ok := idx.next(e.rng, result, &cfg.orderProbs)
		if !ok {
			break
		}
		result = append(result, next)
	}
	return strings.Join(result, " ")
}

// Bridge returns the longest suffix of contextText that appears as a prefix
// in the model, suitable for use as a seed. It returns an empty string if no
// usable bridge exists.
func (e *Engine) Bridge(transitions []Transition, contextText string) string {
	e.mu.Lock()
	defer e.mu.Unlock()
	idx := newIndex(transitions)
	return bridge(idx, contextText)
}

func bridge(idx *index, contextText string) string {
	words := strings.Fields(contextText)
	if len(words) == 0 {
		return ""
	}
	for l := maxOrder; l >= 1; l-- {
		if len(words) < l {
			continue
		}
		suffix := words[len(words)-l:]
		if idx.hasPrefix(suffix) {
			return strings.Join(suffix, " ")
		}
	}
	return ""
}

// stylizeEmotes is a curated list of Russian-friendly reactions appended to
// generated text to give it emotional weight. Swearing lives in swearify.
var stylizeEmotes = []string{
	"😂", "🤡", "😏", "🔥", "💀", "😳", "🤌", "🙃", "😅", "😭",
	"кек", "лол", "хз", "упс", "жесть", "мда", "хых", "бывает",
	"T_T", "...", "ну...", "да ладно", "оно самое", "вот это поворот",
}

// swearInfix is injected between words ("Я блять ...").
var swearInfix = []string{"бля", "блять", "блин", "сука", "жесть", "реально", "в натуре", "пизда"}

// swearTrailing is appended at the end of the sentence ("... нахуй!").
var swearTrailing = []string{"нахуй", "нахер", "пиздец", "ёб твою мать", "в пизду", "твою ж мать", "мать твою", "ёбаный в рот", "в жопу"}

// iqInfix is injected between words as the high-register counterpart to
// swearInfix ("Я концептуально закончил ...").
var iqInfix = []string{
	"концептуально", "эмпирически", "гипотетически", "теоретически",
	"системно", "объективно", "прагматично", "парадоксально",
	"диалектически", "метафорически",
}

// iqTrailing is appended at the end of the sentence ("... если быть точным").
var iqTrailing = []string{
	"собственно говоря", "вообще говоря", "строго говоря", "если быть точным",
	"с научной точки зрения", "безусловно", "впрочем, неважно",
	"что и требовалось доказать", "вообще-то", "как известно",
}

// Styling controls which post-processing passes are applied to generated
// text. The zero value disables everything (no-op). Use DefaultStyling for a
// fully-styled config.
type Styling struct {
	// Caps uppercases a single significant word ("привет" -> "ПРИВЕТ").
	Caps bool `json:"caps"`
	// AllCaps uppercases a leading phrase.
	AllCaps bool `json:"allcaps"`
	// Punct adds energetic trailing punctuation ("...", "!", "!!").
	Punct bool `json:"punct"`
	// Emotes appends reactions / kaomoji ("😂", "кек", "лол").
	Emotes bool `json:"emotes"`
	// Swearify injects Russian profanity.
	Swearify bool `json:"swearify"`
	// Intel injects high-register "intellectual" filler.
	Intel bool `json:"intel"`
}

// DefaultStyling enables every stylization pass.
func DefaultStyling() Styling {
	return Styling{
		Caps: true, AllCaps: true, Punct: true, Emotes: true,
		Swearify: true, Intel: true,
	}
}

// Stylize applies light, probabilistic post-processing to add emotional
// weight: an uppercase word, an ALL CAPS phrase, energetic punctuation,
// reactions, swearing and intellectual filler — as gated by s. It never
// removes or reorders words.
func (e *Engine) Stylize(text string, s Styling) string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return stylize(text, s, e.rng)
}

func stylize(text string, s Styling, r *rand.Rand) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return text
	}

	// 1. Uppercase a single significant word (~30%).
	if s.Caps && r.Float64() < 0.30 {
		var idxs []int
		for i, w := range words {
			if utf8.RuneCountInString(w) > 3 {
				idxs = append(idxs, i)
			}
		}
		if len(idxs) > 0 {
			i := idxs[r.Intn(len(idxs))]
			words[i] = strings.ToUpper(words[i])
		}
	}

	result := strings.Join(words, " ")

	// 2. ALL CAPS a leading phrase (~12%).
	if s.AllCaps && r.Float64() < 0.12 && len(words) > 1 {
		n := 1 + r.Intn(2)
		if n > len(words) {
			n = len(words)
		}
		for i := 0; i < n; i++ {
			words[i] = strings.ToUpper(words[i])
		}
		result = strings.Join(words, " ")
	}

	// 3. Energetic trailing punctuation (~30%).
	if s.Punct && r.Float64() < 0.30 {
		result = strings.TrimRight(result, " \t\n.!?")
		switch r.Intn(3) {
		case 0:
			result += "..."
		case 1:
			result += "!"
		default:
			result += "!!"
		}
	}

	// 4. Trailing reaction (~18%).
	if s.Emotes && r.Float64() < 0.18 {
		result += " " + stylizeEmotes[r.Intn(len(stylizeEmotes))]
	}

	// 5. Register injection: both registers can fire so swearing and
	// intellectual filler mix freely.
	if s.Swearify {
		result = swearify(result, r)
	}
	if s.Intel {
		result = intellectualize(result, r)
	}

	return result
}

// injectRegister inserts words from infix/trailing into text at a few natural
// slots: right after the first word ("Я <infix> закончил ..."), at a random
// mid-sentence position, and/or as a trailing emphasis ("... <trailing>!").
// Each slot is independent, so intensity varies naturally from nothing to all
// three.
func injectRegister(text string, r *rand.Rand, infix, trailing []string, infixProb, midProb, trailProb float64) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return text
	}

	// Detach trailing punctuation so injected words land before it.
	body := strings.TrimRight(trimmed, "!?.,;:…")
	punct := trimmed[len(body):]

	words := strings.Fields(body)
	if len(words) == 0 {
		return text
	}

	// 1. Post-subject insertion: "Я блять закончил..."
	if len(words) > 1 && r.Float64() < infixProb {
		words = insertWord(words, 1, infix[r.Intn(len(infix))])
	}

	// 2. Mid-sentence insertion at a random position.
	if len(words) > 3 && r.Float64() < midProb {
		pos := 2 + r.Intn(len(words)-2)
		words = insertWord(words, pos, infix[r.Intn(len(infix))])
	}

	// 3. Trailing emphasis: "... нахуй!"
	if r.Float64() < trailProb {
		words = append(words, trailing[r.Intn(len(trailing))])
	}

	return strings.Join(words, " ") + punct
}

// swearify injects Russian profanity into text. See injectRegister.
func swearify(text string, r *rand.Rand) string {
	return injectRegister(text, r, swearInfix, swearTrailing, 0.20, 0.15, 0.25)
}

// intellectualize injects high-register, "intellectual" Russian filler — the
// well-spoken counterpart to swearify. See injectRegister.
func intellectualize(text string, r *rand.Rand) string {
	return injectRegister(text, r, iqInfix, iqTrailing, 0.20, 0.15, 0.25)
}

// insertWord inserts w at position pos in words, returning the extended slice.
func insertWord(words []string, pos int, w string) []string {
	words = append(words, "")
	copy(words[pos+1:], words[pos:])
	words[pos] = w
	return words
}
