package encryption

import (
	"testing"
)

func TestAesSimpleEncrypt(t *testing.T) {
	data := "Hello World!"
	keys := []string{
		"1234",
		"16bit secret key",
		"16bit secret key1234567",
		"16bit secret key12345678",
		"16bit secret key16bit secret ke",
		"16bit secret key16bit secret key",
		"16bit secret key16bit secret key1",
	}
	res := []string{
		"NHlpzbcTvOj686VaF7fU7g==",
		"PuMhKY8ZFLnDAwlQ7v/2SQ==",
		"ZG9JUBvEXrXwSS2RIHvpog==",
		"pbvDuBOV3tJrlPV0xdmbKQ==",
		"uAeg71zBzFeUfEMHJqCSxw==",
		"j9SbFFEEFX4dT9VaDAzsCg==",
		"j9SbFFEEFX4dT9VaDAzsCg==",
	}
	for i, key := range keys {
		if got := AesSimpleEncrypt(data, key); got != res[i] {
			t.Errorf("key = %s AesSimpleEncrypt() = %s, want %v", key, got, res[i])
		}
	}
}

func TestAesSimpleDecrypt(t *testing.T) {
	data := "Hello World!"
	keys := []string{
		"1234",
		"16bit secret key",
		"16bit secret key1234567",
		"16bit secret key12345678",
		"16bit secret key16bit secret ke",
		"16bit secret key16bit secret key",
		"16bit secret key16bit secret key1",
	}
	res := []string{
		"NHlpzbcTvOj686VaF7fU7g==",
		"PuMhKY8ZFLnDAwlQ7v/2SQ==",
		"ZG9JUBvEXrXwSS2RIHvpog==",
		"pbvDuBOV3tJrlPV0xdmbKQ==",
		"uAeg71zBzFeUfEMHJqCSxw==",
		"j9SbFFEEFX4dT9VaDAzsCg==",
		"j9SbFFEEFX4dT9VaDAzsCg==",
	}
	for i, key := range keys {
		if got := AesSimpleDecrypt(res[i], key); got != data {
			t.Errorf("key = %s AesSimpleEncrypt() = %s, want %v", key, got, data)
		}
	}
}

func TestGenIVFromKey(t *testing.T) {
	type args struct {
		key string
	}
	tests := []struct {
		name   string
		args   args
		wantIv string
	}{
		{
			name: "test",
			args: args{
				key: "16bit secret key",
			},
			wantIv: "ba79295cdabd3a86",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotIv := GenIVFromKey(tt.args.key); gotIv != tt.wantIv {
				t.Errorf("GenIVFromKey() = %v, want %v", gotIv, tt.wantIv)
			}
		})
	}
}

func TestAesEncrypt(t *testing.T) {
	type args struct {
		data        string
		key         string
		iv          string
		paddingMode PaddingMode
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "test",
			args: args{
				data:        "Hello World!",
				key:         "16bit secret key",
				iv:          "ba79295cdabd3a86",
				paddingMode: PKCS7,
			},
			want: "PuMhKY8ZFLnDAwlQ7v/2SQ==",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := AesCBCEncrypt(tt.args.data, tt.args.key, tt.args.iv, tt.args.paddingMode); got != tt.want {
				t.Errorf("AesEncrypt() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAesDecrypt(t *testing.T) {
	type args struct {
		data        string
		key         string
		iv          string
		paddingMode PaddingMode
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "test",
			args: args{
				data:        "PuMhKY8ZFLnDAwlQ7v/2SQ==",
				key:         "16bit secret key",
				iv:          "ba79295cdabd3a86",
				paddingMode: PKCS7,
			},
			want: "Hello World!",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := AesCBCDecrypt(tt.args.data, tt.args.key, tt.args.iv, tt.args.paddingMode); got != tt.want {
				t.Errorf("AesDecrypt() = %v, want %v", got, tt.want)
			}
		})
	}
}
