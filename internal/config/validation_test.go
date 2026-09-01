package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidatePort(t *testing.T) {
	tests := []struct {
		name      string
		port      int
		portName  string
		wantError bool
	}{
		{"valid port 80", 80, "HTTP", false},
		{"valid port 443", 443, "HTTPS", false},
		{"valid port 1025", 1025, "SMTP", false},
		{"valid port 65535", 65535, "Max", false},
		{"invalid port 0", 0, "Zero", true},
		{"invalid port -1", -1, "Negative", true},
		{"invalid port 65536", 65536, "Over max", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePort(tt.port, tt.portName)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidatePort(%d, %q) error = %v, wantError %v", tt.port, tt.portName, err, tt.wantError)
			}
		})
	}
}

func TestValidateLogLevel(t *testing.T) {
	tests := []struct {
		name      string
		level     string
		wantError bool
	}{
		{"valid silent", "silent", false},
		{"valid normal", "normal", false},
		{"valid verbose", "verbose", false},
		{"valid Silent (case insensitive)", "Silent", false},
		{"valid NORMAL (case insensitive)", "NORMAL", false},
		{"valid Verbose (case insensitive)", "Verbose", false},
		{"invalid debug", "debug", true},
		{"invalid empty", "", true},
		{"invalid info", "info", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateLogLevel(tt.level)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateLogLevel(%q) error = %v, wantError %v", tt.level, err, tt.wantError)
			}
		})
	}
}

func TestValidatePath(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		wantError bool
	}{
		{"valid absolute path", "/tmp/test", false},
		{"valid relative path", "test/file.txt", false},
		{"empty path", "", true},
		{"path traversal", "../../../etc/passwd", true},
		{"path with double dot", "some/../path", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidatePath(tt.path)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidatePath(%q) error = %v, wantError %v", tt.path, err, tt.wantError)
			}
		})
	}
}

func TestValidateMailDir(t *testing.T) {
	tests := []struct {
		name      string
		mailDir   string
		wantError bool
	}{
		{"empty (in-memory)", "", false},
		{"valid path", "/tmp/mail", false},
		{"path traversal", "../../../tmp", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMailDir(tt.mailDir)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateMailDir(%q) error = %v, wantError %v", tt.mailDir, err, tt.wantError)
			}
		})
	}
}

func TestValidateConfig(t *testing.T) {
	t.Run("nil config", func(t *testing.T) {
		err := ValidateConfig(nil)
		if err == nil {
			t.Error("ValidateConfig(nil) should return error")
		}
	})

	t.Run("valid default config", func(t *testing.T) {
		cfg := DefaultConfig()
		err := ValidateConfig(cfg)
		if err != nil {
			t.Errorf("ValidateConfig(DefaultConfig()) error = %v, want nil", err)
		}
	})

	t.Run("invalid SMTP port", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.SMTPPort = 0
		err := ValidateConfig(cfg)
		if err == nil {
			t.Error("ValidateConfig with invalid SMTP port should return error")
		}
	})

	t.Run("invalid SMTP max message size", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.SMTPMaxMessageMB = 0
		if err := ValidateConfig(cfg); err == nil {
			t.Error("zero SMTP max message size should fail")
		}
	})

	t.Run("invalid Web port", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.WebPort = 70000
		err := ValidateConfig(cfg)
		if err == nil {
			t.Error("ValidateConfig with invalid Web port should return error")
		}
	})

	t.Run("invalid outgoing port", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.OutgoingHost = "smtp.example.com"
		cfg.OutgoingPort = -1
		err := ValidateConfig(cfg)
		if err == nil {
			t.Error("ValidateConfig with invalid outgoing port should return error")
		}
	})

	t.Run("invalid log level", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.LogLevel = "invalid"
		err := ValidateConfig(cfg)
		if err == nil {
			t.Error("ValidateConfig with invalid log level should return error")
		}
	})

	t.Run("TLS enabled without cert file", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.TLSEnabled = true
		cfg.TLSCertFile = ""
		cfg.TLSKeyFile = "/path/to/key.pem"
		err := ValidateConfig(cfg)
		if err == nil {
			t.Error("ValidateConfig with TLS enabled but no cert file should return error")
		}
	})

	t.Run("TLS enabled without key file", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.TLSEnabled = true
		cfg.TLSCertFile = "/path/to/cert.pem"
		cfg.TLSKeyFile = ""
		err := ValidateConfig(cfg)
		if err == nil {
			t.Error("ValidateConfig with TLS enabled but no key file should return error")
		}
	})

	t.Run("HTTPS enabled without cert file", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.HTTPSEnabled = true
		cfg.HTTPSCertFile = ""
		cfg.HTTPSKeyFile = "/path/to/key.pem"
		err := ValidateConfig(cfg)
		if err == nil {
			t.Error("ValidateConfig with HTTPS enabled but no cert file should return error")
		}
	})

	t.Run("HTTPS enabled without key file", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.HTTPSEnabled = true
		cfg.HTTPSCertFile = "/path/to/cert.pem"
		cfg.HTTPSKeyFile = ""
		err := ValidateConfig(cfg)
		if err == nil {
			t.Error("ValidateConfig with HTTPS enabled but no key file should return error")
		}
	})

	t.Run("auto relay rules with path traversal", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.AutoRelayRules = "../../../etc/passwd"
		err := ValidateConfig(cfg)
		if err == nil {
			t.Error("ValidateConfig with path traversal in auto relay rules should return error")
		}
	})

	t.Run("webhook config with path traversal", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.WebhookConfig = "../../../etc/passwd"
		err := ValidateConfig(cfg)
		if err == nil {
			t.Error("ValidateConfig with path traversal in webhook config should return error")
		}
	})

	t.Run("negative webhook concurrency", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.WebhookMaxConcurrency = -1
		if err := ValidateConfig(cfg); err == nil {
			t.Error("ValidateConfig with negative webhook concurrency should return error")
		}
	})

	t.Run("invalid storage policy", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.MailMaxMessages = -1
		if err := ValidateConfig(cfg); err == nil {
			t.Error("negative mail max messages should fail")
		}
		cfg = DefaultConfig()
		cfg.MailCleanupInterval = "never"
		if err := ValidateConfig(cfg); err == nil {
			t.Error("invalid mail cleanup interval should fail")
		}
	})

	t.Run("invalid webhook drain settings", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.WebhookShutdownTimeout = "never"
		if err := ValidateConfig(cfg); err == nil {
			t.Error("invalid webhook shutdown timeout should fail")
		}
		cfg = DefaultConfig()
		cfg.WebhookRedisPrefix = " "
		if err := ValidateConfig(cfg); err == nil {
			t.Error("empty webhook Redis prefix should fail")
		}
		cfg = DefaultConfig()
		cfg.WebExternalScheme = "ftp"
		if err := ValidateConfig(cfg); err == nil {
			t.Error("invalid external web scheme should fail")
		}
	})

	t.Run("S3 attachment storage", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.S3Enabled = true
		if err := ValidateConfig(cfg); err == nil {
			t.Error("enabled S3 storage without a bucket should fail")
		}

		cfg.S3Bucket = "owlmail"
		cfg.S3Region = ""
		if err := ValidateConfig(cfg); err == nil {
			t.Error("enabled S3 storage without a region should fail")
		}

		cfg.S3Region = "us-east-1"
		cfg.S3Endpoint = "ftp://objects.example.test"
		if err := ValidateConfig(cfg); err == nil {
			t.Error("non-HTTP S3 endpoint should fail")
		}

		cfg.S3Endpoint = "https://user:pass@objects.example.test"
		if err := ValidateConfig(cfg); err == nil {
			t.Error("S3 endpoint credentials should fail")
		}

		cfg.S3Endpoint = "http://minio:9000"
		cfg.S3AccessKeyID = "access"
		if err := ValidateConfig(cfg); err == nil {
			t.Error("partial static S3 credentials should fail")
		}

		cfg.S3SecretAccessKey = "secret"
		if err := ValidateConfig(cfg); err != nil {
			t.Fatalf("valid S3 attachment config error = %v", err)
		}
	})

	t.Run("valid full config", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.SMTPPort = 2525
		cfg.WebPort = 8080
		cfg.OutgoingHost = "smtp.example.com"
		cfg.OutgoingPort = 587
		cfg.LogLevel = "verbose"
		err := ValidateConfig(cfg)
		if err != nil {
			t.Errorf("ValidateConfig with valid full config error = %v, want nil", err)
		}
	})
}

func TestValidateFileExists(t *testing.T) {
	// Create a temporary file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(tmpFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	t.Run("existing file", func(t *testing.T) {
		err := ValidateFileExists(tmpFile)
		if err != nil {
			t.Errorf("ValidateFileExists for existing file error = %v, want nil", err)
		}
	})

	t.Run("non-existing file", func(t *testing.T) {
		err := ValidateFileExists(filepath.Join(tmpDir, "nonexistent.txt"))
		if err == nil {
			t.Error("ValidateFileExists for non-existing file should return error")
		}
	})

	t.Run("directory instead of file", func(t *testing.T) {
		err := ValidateFileExists(tmpDir)
		if err == nil {
			t.Error("ValidateFileExists for directory should return error")
		}
	})
}

func TestValidateDirExists(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(tmpFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	t.Run("existing directory", func(t *testing.T) {
		err := ValidateDirExists(tmpDir)
		if err != nil {
			t.Errorf("ValidateDirExists for existing directory error = %v, want nil", err)
		}
	})

	t.Run("non-existing directory", func(t *testing.T) {
		err := ValidateDirExists(filepath.Join(tmpDir, "nonexistent"))
		if err == nil {
			t.Error("ValidateDirExists for non-existing directory should return error")
		}
	})

	t.Run("file instead of directory", func(t *testing.T) {
		err := ValidateDirExists(tmpFile)
		if err == nil {
			t.Error("ValidateDirExists for file should return error")
		}
	})
}

func TestValidateDirWritable(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("writable directory", func(t *testing.T) {
		err := ValidateDirWritable(tmpDir)
		if err != nil {
			t.Errorf("ValidateDirWritable for writable directory error = %v, want nil", err)
		}
	})

	t.Run("non-existing directory", func(t *testing.T) {
		err := ValidateDirWritable(filepath.Join(tmpDir, "nonexistent"))
		if err == nil {
			t.Error("ValidateDirWritable for non-existing directory should return error")
		}
	})
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"valid silent", "silent", "silent"},
		{"valid normal", "normal", "normal"},
		{"valid verbose", "verbose", "verbose"},
		{"invalid returns normal", "invalid", "normal"},
		{"empty returns normal", "", "normal"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseLogLevel(tt.input)
			if result != tt.expected {
				t.Errorf("ParseLogLevel(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestValidateTLSFiles(t *testing.T) {
	tmpDir := t.TempDir()
	certFile := filepath.Join(tmpDir, "cert.pem")
	keyFile := filepath.Join(tmpDir, "key.pem")

	// Create test files
	if err := os.WriteFile(certFile, []byte("cert"), 0644); err != nil {
		t.Fatalf("Failed to create cert file: %v", err)
	}
	if err := os.WriteFile(keyFile, []byte("key"), 0644); err != nil {
		t.Fatalf("Failed to create key file: %v", err)
	}

	t.Run("valid files", func(t *testing.T) {
		err := ValidateTLSFiles(certFile, keyFile)
		if err != nil {
			t.Errorf("ValidateTLSFiles for valid files error = %v, want nil", err)
		}
	})

	t.Run("missing cert file", func(t *testing.T) {
		err := ValidateTLSFiles(filepath.Join(tmpDir, "missing.pem"), keyFile)
		if err == nil {
			t.Error("ValidateTLSFiles with missing cert file should return error")
		}
	})

	t.Run("missing key file", func(t *testing.T) {
		err := ValidateTLSFiles(certFile, filepath.Join(tmpDir, "missing.pem"))
		if err == nil {
			t.Error("ValidateTLSFiles with missing key file should return error")
		}
	})
}

func TestValidateHTTPSFiles(t *testing.T) {
	tmpDir := t.TempDir()
	certFile := filepath.Join(tmpDir, "https-cert.pem")
	keyFile := filepath.Join(tmpDir, "https-key.pem")

	// Create test files
	if err := os.WriteFile(certFile, []byte("cert"), 0644); err != nil {
		t.Fatalf("Failed to create cert file: %v", err)
	}
	if err := os.WriteFile(keyFile, []byte("key"), 0644); err != nil {
		t.Fatalf("Failed to create key file: %v", err)
	}

	t.Run("valid files", func(t *testing.T) {
		err := ValidateHTTPSFiles(certFile, keyFile)
		if err != nil {
			t.Errorf("ValidateHTTPSFiles for valid files error = %v, want nil", err)
		}
	})

	t.Run("missing cert file", func(t *testing.T) {
		err := ValidateHTTPSFiles(filepath.Join(tmpDir, "missing.pem"), keyFile)
		if err == nil {
			t.Error("ValidateHTTPSFiles with missing cert file should return error")
		}
	})

	t.Run("missing key file", func(t *testing.T) {
		err := ValidateHTTPSFiles(certFile, filepath.Join(tmpDir, "missing.pem"))
		if err == nil {
			t.Error("ValidateHTTPSFiles with missing key file should return error")
		}
	})
}

func TestValidatePathWithOptions(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("valid path with allowed dir", func(t *testing.T) {
		path := filepath.Join(tmpDir, "test.txt")
		absPath, err := ValidatePathWithOptions(path, true, []string{tmpDir})
		if err != nil {
			t.Errorf("ValidatePathWithOptions error = %v, want nil", err)
		}
		if absPath == "" {
			t.Error("ValidatePathWithOptions should return absolute path")
		}
	})

	t.Run("empty path", func(t *testing.T) {
		_, err := ValidatePathWithOptions("", true, nil)
		if err == nil {
			t.Error("ValidatePathWithOptions with empty path should return error")
		}
	})

	t.Run("path traversal blocked", func(t *testing.T) {
		_, err := ValidatePathWithOptions("../../../etc/passwd", true, nil)
		if err == nil {
			t.Error("ValidatePathWithOptions with path traversal should return error")
		}
	})
}
