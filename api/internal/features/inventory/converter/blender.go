package converter

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

//go:embed avaturn_to_vrm.py
var script []byte

//go:embed avaturn_profile.json
var profileJSON []byte

var ErrUnavailable = errors.New("avatar converter unavailable")

type Asset struct {
	*os.File
	directory string
}

func (a *Asset) Close() error { return errors.Join(a.File.Close(), os.RemoveAll(a.directory)) }

type Blender struct {
	busy            atomic.Bool
	Binary          string
	Timeout         time.Duration
	MaxOutput       int64
	CommercialUsage string
	LicenseURL      string
}

func (b *Blender) Available(ctx context.Context) error {
	path, err := exec.LookPath(b.Binary)
	if err != nil {
		return ErrUnavailable
	}
	checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	code := `import addon_utils,bpy,tomllib; from pathlib import Path; m=addon_utils.enable("bl_ext.user_default.vrm",default_set=True); assert m is not None; assert bpy.app.version[:2]==(5,2); assert tomllib.loads(Path(m.__file__).with_name("blender_manifest.toml").read_text())["version"]=="4.10.2"; bpy.ops.export_scene.vrm.get_rna_type()`
	cmd := exec.CommandContext(checkCtx, path, "--background", "--factory-startup", "--disable-autoexec", "--python-exit-code", "1", "--python-expr", code)
	cmd.Env = processEnvironment()
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if cmd.Run() != nil {
		return ErrUnavailable
	}
	return nil
}

func (b *Blender) Convert(ctx context.Context, source io.ReadSeeker, name string) (*Asset, error) {
	// One Blender process per API instance; reject overload without queuing uploads.
	if !b.busy.CompareAndSwap(false, true) {
		return nil, ErrUnavailable
	}
	defer b.busy.Store(false)
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	directory, err := os.MkdirTemp("", "rolecue-avatar-")
	if err != nil {
		return nil, err
	}
	success := false
	defer func() {
		if !success {
			_ = os.RemoveAll(directory)
		}
	}()
	input := filepath.Join(directory, "source.glb")
	output := filepath.Join(directory, "model.vrm")
	if _, err = source.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(input, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	_, copyErr := io.Copy(f, source)
	closeErr := f.Close()
	if err = errors.Join(copyErr, closeErr); err != nil {
		return nil, err
	}
	scriptPath := filepath.Join(directory, "convert.py")
	if err = os.WriteFile(scriptPath, script, 0600); err != nil {
		return nil, err
	}
	if err = os.WriteFile(filepath.Join(directory, "avaturn_profile.json"), profileJSON, 0600); err != nil {
		return nil, err
	}
	metadata, _ := json.Marshal(map[string]string{"name": name, "commercial_usage": b.CommercialUsage, "license_url": b.LicenseURL})
	metadataPath := filepath.Join(directory, "metadata.json")
	if err = os.WriteFile(metadataPath, metadata, 0600); err != nil {
		return nil, err
	}
	processCtx, cancel := context.WithTimeout(ctx, b.Timeout)
	defer cancel()
	cmd := exec.CommandContext(processCtx, b.Binary, "--background", "--factory-startup", "--disable-autoexec", "--python-exit-code", "1", "--python", scriptPath, "--", "--input", input, "--output", output, "--metadata", metadataPath)
	cmd.Env = processEnvironment()
	cmd.Stdout = io.Discard
	stderr := &tailWriter{limit: 4 << 10}
	cmd.Stderr = stderr
	if err = cmd.Run(); err != nil {
		if processCtx.Err() != nil {
			return nil, processCtx.Err()
		}
		return nil, fmt.Errorf("VRM exporter failed: %w: %s", err, bytes.TrimSpace(stderr.data))
	}
	info, err := os.Stat(output)
	if err != nil {
		return nil, err
	}
	if info.Size() <= 0 || info.Size() > b.MaxOutput {
		return nil, fmt.Errorf("VRM output exceeds size policy")
	}
	result, err := os.Open(output)
	if err != nil {
		return nil, err
	}
	if _, err = Validate(result, info.Size(), true); err != nil {
		_ = result.Close()
		return nil, err
	}
	if _, err = result.Seek(0, io.SeekStart); err != nil {
		_ = result.Close()
		return nil, err
	}
	success = true
	return &Asset{File: result, directory: directory}, nil
}

func processEnvironment() []string {
	var result []string
	for _, key := range []string{"PATH", "HOME", "USER", "TMPDIR", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "LANG"} {
		if value, ok := os.LookupEnv(key); ok {
			result = append(result, key+"="+value)
		}
	}
	// No credential-bearing API environment is inherited by the subprocess.
	return append(result, "PYTHONNOUSERSITE=1", "BLENDER_SYSTEM_SCRIPTS="+strings.TrimSpace(os.Getenv("BLENDER_SYSTEM_SCRIPTS")))
}

// tailWriter keeps the end of Blender's stderr, where a failed script prints its Python traceback.
type tailWriter struct {
	data  []byte
	limit int
}

func (w *tailWriter) Write(p []byte) (int, error) {
	w.data = append(w.data, p...)
	if len(w.data) > w.limit {
		w.data = w.data[len(w.data)-w.limit:]
	}
	return len(p), nil
}
