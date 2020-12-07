package sse

import (
	"bytes"
	"testing"
)

func TestEmptyMessage(t *testing.T) {
	msg := Message{}

	if msg.String() != "\n" {
		t.Fatal("Message does not match.")
	}
}

func TestDataMessage(t *testing.T) {
	msg := Message{data: "test"}

	if msg.String() != "data: test\n\n" {
		t.Fatal("Message does not match.")
	}
}

func TestMessage(t *testing.T) {
	msg := Message{
		"123",
		"test",
		"myevent",
		10 * 1000,
	}

	if msg.String() != "id: 123\nretry: 10000\nevent: myevent\ndata: test\n\n" {
		t.Fatal("Message does not match.")
	}
}

func TestMultilineDataMessage(t *testing.T) {
	msg := Message{data: "test\ntest"}

	if msg.String() != "data: test\ndata: test\n\n" {
		t.Fail()
	}
}

func TestSpecialCharacterMessage(t *testing.T) {
	msg := Message{data: "%x%o"}

	if msg.String() != "data: %x%o\n\n" {
		t.Fatal("Message does not match.")
	}
}

func TestMessageBatch(t *testing.T) {
	testData := []struct {
		messages []Message
		expected string
	}{
		{
			nil,
			"",
		},
		{
			[]Message{
				{
					id:    "id1",
					data:  "data1a\ndata1b",
					event: "event1",
				},
				{
					id:    "id2",
					data:  "data2a\ndata2b",
					event: "event2",
				},
			},
			`id: id1
event: event1
data: data1a
data: data1b

id: id2
event: event2
data: data2a
data: data2b

`,
		},
	}

	for i := range testData {
		var buf []byte
		b := bytes.NewBuffer(buf)
		if err := writeMessages(b, testData[i].messages); err != nil {
			t.Fatal(err)
		}
		if b.String() != testData[i].expected {
			t.Fatalf("Expected %q, got %q\n", testData[i].expected, b.String())
		}
	}
}
