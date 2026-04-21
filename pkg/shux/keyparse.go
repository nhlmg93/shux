package shux

import (
	"fmt"
	"strings"
	"unicode"
)

func parseKeySpec(spec string) (string, KeyInput, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return "", KeyInput{}, fmt.Errorf("empty key spec")
	}

	if spec == "-" {
		return "-", KeyInput{Code: '-'}, nil
	}

	tokens := strings.FieldsFunc(spec, func(r rune) bool {
		return r == '-' || r == '+'
	})
	if len(tokens) == 0 {
		return "", KeyInput{}, fmt.Errorf("invalid key spec %q", spec)
	}

	var mods KeyMods
	for _, token := range tokens[:len(tokens)-1] {
		switch strings.ToLower(strings.TrimSpace(token)) {
		case "c", "ctrl", "control":
			mods |= KeyModCtrl
		case "m", "alt", "meta":
			mods |= KeyModAlt
		case "s", "shift":
			mods |= KeyModShift
		case "super":
			mods |= KeyModSuper
		default:
			return "", KeyInput{}, fmt.Errorf("unknown modifier %q", token)
		}
	}

	keyToken := strings.TrimSpace(tokens[len(tokens)-1])
	if keyToken == "" {
		return "", KeyInput{}, fmt.Errorf("missing key name")
	}

	keyName, input, err := parseKeyToken(keyToken, mods)
	if err != nil {
		return "", KeyInput{}, err
	}

	prefixes := make([]string, 0, 5)
	if mods&KeyModCtrl != 0 {
		prefixes = append(prefixes, "ctrl")
	}
	if mods&KeyModAlt != 0 {
		prefixes = append(prefixes, "alt")
	}
	if mods&KeyModShift != 0 && len([]rune(keyName)) != 1 {
		prefixes = append(prefixes, "shift")
	}
	if mods&KeyModMeta != 0 {
		prefixes = append(prefixes, "meta")
	}
	if mods&KeyModSuper != 0 {
		prefixes = append(prefixes, "super")
	}
	if len(prefixes) == 0 {
		return keyName, input, nil
	}
	return strings.Join(append(prefixes, keyName), "+"), input, nil
}

func parseKeyToken(token string, mods KeyMods) (string, KeyInput, error) {
	lower := strings.ToLower(token)
	switch lower {
	case "up":
		return "up", KeyInput{Code: KeyCodeUp, Mods: mods}, nil
	case "down":
		return "down", KeyInput{Code: KeyCodeDown, Mods: mods}, nil
	case "left":
		return "left", KeyInput{Code: KeyCodeLeft, Mods: mods}, nil
	case "right":
		return "right", KeyInput{Code: KeyCodeRight, Mods: mods}, nil
	case "home":
		return "home", KeyInput{Code: KeyCodeHome, Mods: mods}, nil
	case "end":
		return "end", KeyInput{Code: KeyCodeEnd, Mods: mods}, nil
	case "pgup", "pageup":
		return "pgup", KeyInput{Code: KeyCodePageUp, Mods: mods}, nil
	case "pgdown", "pagedown":
		return "pgdown", KeyInput{Code: KeyCodePageDown, Mods: mods}, nil
	case "insert", "ins":
		return "insert", KeyInput{Code: KeyCodeInsert, Mods: mods}, nil
	case "delete", "del":
		return "delete", KeyInput{Code: KeyCodeDelete, Mods: mods}, nil
	case "enter", "return":
		return "enter", KeyInput{Code: KeyCodeEnter, Mods: mods}, nil
	case "backspace", "bs":
		return "backspace", KeyInput{Code: KeyCodeBackspace, Mods: mods}, nil
	case "tab":
		return "tab", KeyInput{Code: KeyCodeTab, Mods: mods}, nil
	case "esc", "escape":
		return "esc", KeyInput{Code: KeyCodeEscape, Mods: mods}, nil
	case "space":
		if mods == 0 {
			return " ", KeyInput{Code: ' '}, nil
		}
		return "space", KeyInput{Code: ' ', Mods: mods}, nil
	}

	if len(lower) >= 2 && lower[0] == 'f' {
		switch lower {
		case "f1":
			return "f1", KeyInput{Code: KeyCodeF1, Mods: mods}, nil
		case "f2":
			return "f2", KeyInput{Code: KeyCodeF2, Mods: mods}, nil
		case "f3":
			return "f3", KeyInput{Code: KeyCodeF3, Mods: mods}, nil
		case "f4":
			return "f4", KeyInput{Code: KeyCodeF4, Mods: mods}, nil
		case "f5":
			return "f5", KeyInput{Code: KeyCodeF5, Mods: mods}, nil
		case "f6":
			return "f6", KeyInput{Code: KeyCodeF6, Mods: mods}, nil
		case "f7":
			return "f7", KeyInput{Code: KeyCodeF7, Mods: mods}, nil
		case "f8":
			return "f8", KeyInput{Code: KeyCodeF8, Mods: mods}, nil
		case "f9":
			return "f9", KeyInput{Code: KeyCodeF9, Mods: mods}, nil
		case "f10":
			return "f10", KeyInput{Code: KeyCodeF10, Mods: mods}, nil
		case "f11":
			return "f11", KeyInput{Code: KeyCodeF11, Mods: mods}, nil
		case "f12":
			return "f12", KeyInput{Code: KeyCodeF12, Mods: mods}, nil
		}
	}

	runes := []rune(token)
	if len(runes) != 1 {
		return "", KeyInput{}, fmt.Errorf("unsupported key %q", token)
	}

	r := runes[0]
	if unicode.IsLetter(r) {
		if unicode.IsUpper(r) {
			mods |= KeyModShift
		}
		r = unicode.ToLower(r)
	}

	input := KeyInput{Code: r, Mods: mods}
	return string(runes), input, nil
}
