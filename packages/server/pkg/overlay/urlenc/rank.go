package urlenc

import (
    "math/big"
    "strings"
)

// Simple fixed-width base36 rank strings. Lex order == numeric order.
// Width 16 gives ~2e25 space; practically no saturation.
const rankWidth = 16
const base36 = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"

var b36 = big.NewInt(36)

func parseRank(s string) *big.Int {
    if s == "" { return big.NewInt(0) }
    n := big.NewInt(0)
    for _, r := range s {
        idx := strings.IndexRune(base36, r)
        if idx < 0 { idx = 0 }
        n.Mul(n, b36).Add(n, big.NewInt(int64(idx)))
    }
    return n
}

func formatRank(n *big.Int) string {
    if n.Sign() < 0 { n = big.NewInt(0) }
    // convert to base36 string
    if n.Sign() == 0 { return strings.Repeat("0", rankWidth) }
    var digits []byte
    tmp := new(big.Int).Set(n)
    for tmp.Sign() > 0 {
        mod := new(big.Int)
        tmp.DivMod(tmp, b36, mod)
        digits = append(digits, base36[mod.Int64()])
    }
    // reverse
    for i, j := 0, len(digits)-1; i < j; i, j = i+1, j-1 {
        digits[i], digits[j] = digits[j], digits[i]
    }
    s := string(digits)
    if len(s) < rankWidth {
        s = strings.Repeat("0", rankWidth-len(s)) + s
    } else if len(s) > rankWidth {
        // clamp to width
        s = s[:rankWidth]
    }
    return s
}

func maxRank() *big.Int {
    // base^width - 1
    pow := new(big.Int).Exp(b36, big.NewInt(rankWidth), nil)
    return pow.Sub(pow, big.NewInt(1))
}

// RankBetween returns a rank string strictly between prev and next.
// If prev == "" it treats prev = 0; if next == "" it treats next = max.
func RankBetween(prev, next string) string {
    var a = big.NewInt(0)
    var b = maxRank()
    if prev != "" { a = parseRank(prev) }
    if next != "" { b = parseRank(next) }
    sum := new(big.Int).Add(a, b)
    mid := sum.Rsh(sum, 1) // divide by 2
    // ensure strictly between by nudging if equal to endpoints
    if prev != "" && mid.Cmp(a) == 0 {
        mid = new(big.Int).Add(a, big.NewInt(1))
    }
    if next != "" && mid.Cmp(b) == 0 {
        mid = new(big.Int).Sub(b, big.NewInt(1))
    }
    return formatRank(mid)
}

// FirstRank returns an initial middle rank.
func FirstRank() string {
    return RankBetween("", "")
}

