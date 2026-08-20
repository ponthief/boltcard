package crypto

import (
	"encoding/hex"
	"testing"
)

// test vectors taken from docs/TEST_VECTORS.md, produced with a real card

type card_vector struct {
	p               string
	c               string
	aes_decrypt_key string
	aes_cmac_key    string
	uid             string
	ctr             uint32
}

var card_vectors = []card_vector{
	{
		p:               "4E2E289D945A66BB13377A728884E867",
		c:               "E19CCB1FED8892CE",
		aes_decrypt_key: "0c3b25d92b38ae443229dd59ad34b85d",
		aes_cmac_key:    "b45775776cb224c75bcde7ca3704e933",
		uid:             "04996c6a926980",
		ctr:             3,
	},
	{
		p:               "00F48C4F8E386DED06BCDC78FA92E2FE",
		c:               "66B4826EA4C155B4",
		aes_decrypt_key: "0c3b25d92b38ae443229dd59ad34b85d",
		aes_cmac_key:    "b45775776cb224c75bcde7ca3704e933",
		uid:             "04996c6a926980",
		ctr:             5,
	},
	{
		p:               "0DBF3C59B59B0638D60B5842A997D4D1",
		c:               "CC61660C020B4D96",
		aes_decrypt_key: "0c3b25d92b38ae443229dd59ad34b85d",
		aes_cmac_key:    "b45775776cb224c75bcde7ca3704e933",
		uid:             "04996c6a926980",
		ctr:             7,
	},
}

// sv2 as built by check_cmac in the lnurlw package
func sv2_for(uid []byte, ctr []byte) []byte {
	return []byte{0x3c, 0xc3, 0x00, 0x01, 0x00, 0x80,
		uid[0], uid[1], uid[2], uid[3], uid[4], uid[5], uid[6],
		ctr[0], ctr[1], ctr[2]}
}

func Test_aes_decrypt_card_data(t *testing.T) {
	for _, v := range card_vectors {
		key, _ := hex.DecodeString(v.aes_decrypt_key)
		ba_p, _ := hex.DecodeString(v.p)

		dec_p, err := Aes_decrypt(key, ba_p)
		if err != nil {
			t.Fatalf("Aes_decrypt(%s) errored: %v", v.p, err)
		}

		if dec_p[0] != 0xC7 {
			t.Errorf("p %s: decrypted data starts with %#x, want 0xc7", v.p, dec_p[0])
		}

		uid := hex.EncodeToString(dec_p[1:8])
		if uid != v.uid {
			t.Errorf("p %s: uid = %s, want %s", v.p, uid, v.uid)
		}

		ctr := dec_p[8:11]
		ctr_int := uint32(ctr[2])<<16 | uint32(ctr[1])<<8 | uint32(ctr[0])
		if ctr_int != v.ctr {
			t.Errorf("p %s: ctr = %d, want %d", v.p, ctr_int, v.ctr)
		}
	}
}

func Test_aes_cmac_accepts_a_card_cmac(t *testing.T) {
	for _, v := range card_vectors {
		decrypt_key, _ := hex.DecodeString(v.aes_decrypt_key)
		cmac_key, _ := hex.DecodeString(v.aes_cmac_key)
		ba_p, _ := hex.DecodeString(v.p)
		ba_c, _ := hex.DecodeString(v.c)

		dec_p, _ := Aes_decrypt(decrypt_key, ba_p)
		sv2 := sv2_for(dec_p[1:8], dec_p[8:11])

		valid, err := Aes_cmac(cmac_key, sv2, ba_c)
		if err != nil {
			t.Fatalf("Aes_cmac errored for p %s: %v", v.p, err)
		}

		if !valid {
			t.Errorf("p %s: a valid card cmac was rejected", v.p)
		}
	}
}

func Test_aes_cmac_rejects_a_wrong_cmac(t *testing.T) {
	v := card_vectors[0]

	decrypt_key, _ := hex.DecodeString(v.aes_decrypt_key)
	cmac_key, _ := hex.DecodeString(v.aes_cmac_key)
	ba_p, _ := hex.DecodeString(v.p)

	dec_p, _ := Aes_decrypt(decrypt_key, ba_p)
	sv2 := sv2_for(dec_p[1:8], dec_p[8:11])

	tests := []struct {
		name string
		c    string
	}{
		{"last byte changed", "E19CCB1FED8892CF"},
		{"first byte changed", "E29CCB1FED8892CE"},
		{"all zeroes", "0000000000000000"},
		{"cmac from another counter value", "66B4826EA4C155B4"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ba_c, _ := hex.DecodeString(test.c)

			valid, err := Aes_cmac(cmac_key, sv2, ba_c)
			if err != nil {
				t.Fatalf("Aes_cmac errored: %v", err)
			}

			if valid {
				t.Errorf("cmac %s was accepted", test.c)
			}
		})
	}
}

func Test_aes_cmac_rejects_a_wrong_key(t *testing.T) {
	v := card_vectors[0]

	decrypt_key, _ := hex.DecodeString(v.aes_decrypt_key)
	ba_p, _ := hex.DecodeString(v.p)
	ba_c, _ := hex.DecodeString(v.c)

	dec_p, _ := Aes_decrypt(decrypt_key, ba_p)
	sv2 := sv2_for(dec_p[1:8], dec_p[8:11])

	wrong_key, _ := hex.DecodeString("b45775776cb224c75bcde7ca3704e934")

	valid, err := Aes_cmac(wrong_key, sv2, ba_c)
	if err != nil {
		t.Fatalf("Aes_cmac errored: %v", err)
	}

	if valid {
		t.Error("a cmac was accepted under the wrong key")
	}
}

func Test_create_k1_is_random_and_128_bit(t *testing.T) {
	seen := make(map[string]bool)

	for i := 0; i < 100; i++ {
		k1, err := Create_k1()
		if err != nil {
			t.Fatalf("Create_k1 errored: %v", err)
		}

		if len(k1) != 32 {
			t.Fatalf("k1 length = %d, want 32 hex characters", len(k1))
		}

		if seen[k1] {
			t.Fatalf("Create_k1 returned a repeated value: %s", k1)
		}
		seen[k1] = true
	}
}
