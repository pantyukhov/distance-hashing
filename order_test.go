package distancehashing

import (
	"testing"
)

// TestOrderIndependence проверяет, важен ли порядок операций LinkIdentifiers
func TestOrderIndependence(t *testing.T) {
	tests := []struct {
		name     string
		scenario func(*SessionGenerator) string
	}{
		{
			name: "Scenario A: cookie→uid, затем uid→email",
			scenario: func(sg *SessionGenerator) string {
				sg.LinkIdentifiers("cookie:abc", "uid:user_1")
				sg.LinkIdentifiers("uid:user_1", "email:test@example.com")
				return sg.GetSessionKey(Identifiers{"cookie": "abc"})
			},
		},
		{
			name: "Scenario B: cookie→email, затем email→uid",
			scenario: func(sg *SessionGenerator) string {
				sg.LinkIdentifiers("cookie:abc", "email:test@example.com")
				sg.LinkIdentifiers("email:test@example.com", "uid:user_1")
				return sg.GetSessionKey(Identifiers{"cookie": "abc"})
			},
		},
		{
			name: "Scenario C: uid→email, затем cookie→uid",
			scenario: func(sg *SessionGenerator) string {
				sg.LinkIdentifiers("uid:user_1", "email:test@example.com")
				sg.LinkIdentifiers("cookie:abc", "uid:user_1")
				return sg.GetSessionKey(Identifiers{"cookie": "abc"})
			},
		},
	}

	var sessionKeys []string

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sg, _ := NewSessionGenerator(100)
			key := tt.scenario(sg)
			t.Logf("Session key: %s", key)
			sessionKeys = append(sessionKeys, key)
		})
	}

	// Все ключи должны быть одинаковыми
	t.Log("\n=== Проверка идентичности ключей ===")
	for i := 1; i < len(sessionKeys); i++ {
		if sessionKeys[i] != sessionKeys[0] {
			t.Errorf("❌ ПОРЯДОК ВАЖЕН! Ключи отличаются:\n  Scenario A: %s\n  Scenario %d: %s",
				sessionKeys[0], i+1, sessionKeys[i])
		}
	}

	if len(sessionKeys) > 1 && sessionKeys[0] == sessionKeys[1] && sessionKeys[1] == sessionKeys[2] {
		t.Logf("✅ ПОРЯДОК НЕ ВАЖЕН! Все ключи идентичны: %s", sessionKeys[0])
	}
}

// TestOrderIndependenceWithMultipleComponents проверяет порядок при объединении компонент
func TestOrderIndependenceWithMultipleComponents(t *testing.T) {
	tests := []struct {
		name     string
		scenario func(*SessionGenerator) string
	}{
		{
			name: "Scenario A: Сначала создаем две компоненты, затем линкуем через uid↔email",
			scenario: func(sg *SessionGenerator) string {
				// Компонента 1: {cookie:A, uid:1}
				sg.LinkIdentifiers("cookie:A", "uid:1")
				// Компонента 2: {email:x, device:D}
				sg.LinkIdentifiers("email:x", "device:D")
				// Объединяем через uid↔email
				sg.LinkIdentifiers("uid:1", "email:x")
				return sg.GetSessionKey(Identifiers{"cookie": "A"})
			},
		},
		{
			name: "Scenario B: Сначала создаем две компоненты, затем линкуем через cookie↔device",
			scenario: func(sg *SessionGenerator) string {
				// Компонента 1: {cookie:A, uid:1}
				sg.LinkIdentifiers("cookie:A", "uid:1")
				// Компонента 2: {email:x, device:D}
				sg.LinkIdentifiers("email:x", "device:D")
				// Объединяем через cookie↔device
				sg.LinkIdentifiers("cookie:A", "device:D")
				return sg.GetSessionKey(Identifiers{"cookie": "A"})
			},
		},
		{
			name: "Scenario C: Линкуем все последовательно по цепочке",
			scenario: func(sg *SessionGenerator) string {
				sg.LinkIdentifiers("cookie:A", "uid:1")
				sg.LinkIdentifiers("uid:1", "email:x")
				sg.LinkIdentifiers("email:x", "device:D")
				return sg.GetSessionKey(Identifiers{"cookie": "A"})
			},
		},
		{
			name: "Scenario D: Линкуем в обратном порядке",
			scenario: func(sg *SessionGenerator) string {
				sg.LinkIdentifiers("device:D", "email:x")
				sg.LinkIdentifiers("email:x", "uid:1")
				sg.LinkIdentifiers("uid:1", "cookie:A")
				return sg.GetSessionKey(Identifiers{"cookie": "A"})
			},
		},
	}

	var sessionKeys []string

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sg, _ := NewSessionGenerator(100)
			key := tt.scenario(sg)
			t.Logf("Session key: %s", key)
			sessionKeys = append(sessionKeys, key)

			// Проверяем, что все 4 идентификатора в одной компоненте
			if !sg.AreLinked("cookie:A", "uid:1") {
				t.Error("cookie:A и uid:1 не связаны!")
			}
			if !sg.AreLinked("cookie:A", "email:x") {
				t.Error("cookie:A и email:x не связаны!")
			}
			if !sg.AreLinked("cookie:A", "device:D") {
				t.Error("cookie:A и device:D не связаны!")
			}
		})
	}

	// Все ключи должны быть одинаковыми
	t.Log("\n=== Проверка идентичности ключей при объединении компонент ===")
	allSame := true
	for i := 1; i < len(sessionKeys); i++ {
		if sessionKeys[i] != sessionKeys[0] {
			t.Errorf("❌ ПОРЯДОК ВАЖЕН! Ключи отличаются:\n  Scenario A: %s\n  Scenario %d: %s",
				sessionKeys[0], i+1, sessionKeys[i])
			allSame = false
		}
	}

	if allSame {
		t.Logf("✅ ПОРЯДОК НЕ ВАЖЕН! Все ключи идентичны даже при разных способах объединения компонент: %s", sessionKeys[0])
	}
}

// TestOrderIndependenceInIdentifiersMap проверяет порядок итерации по map
func TestOrderIndependenceInIdentifiersMap(t *testing.T) {
	sg, _ := NewSessionGenerator(100)

	// Создаем идентификаторы в разном порядке добавления в map
	// (map в Go имеет недетерминированный порядок итерации)
	key1 := sg.GetSessionKey(Identifiers{
		"uid":    "user_123",
		"email":  "test@example.com",
		"cookie": "abc",
		"device": "xyz",
	})

	key2 := sg.GetSessionKey(Identifiers{
		"device": "xyz",
		"cookie": "abc",
		"email":  "test@example.com",
		"uid":    "user_123",
	})

	key3 := sg.GetSessionKey(Identifiers{
		"email":  "test@example.com",
		"device": "xyz",
		"uid":    "user_123",
		"cookie": "abc",
	})

	t.Logf("Key 1 (порядок: uid, email, cookie, device): %s", key1)
	t.Logf("Key 2 (порядок: device, cookie, email, uid): %s", key2)
	t.Logf("Key 3 (порядок: email, device, uid, cookie): %s", key3)

	if key1 != key2 || key2 != key3 {
		t.Errorf("❌ ПОРЯДОК КЛЮЧЕЙ В MAP ВАЖЕН!\n  Key1: %s\n  Key2: %s\n  Key3: %s",
			key1, key2, key3)
	} else {
		t.Logf("✅ ПОРЯДОК КЛЮЧЕЙ В MAP НЕ ВАЖЕН! Все ключи идентичны: %s", key1)
	}
}
