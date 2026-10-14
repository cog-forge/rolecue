package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/config"
	"github.com/cog-forge/rolecue/api/internal/features/inventory/converter"
	"github.com/cog-forge/rolecue/api/internal/features/inventory/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/qmuntal/gltf"
	"github.com/qmuntal/gltf/modeler"
)

type creationRepository struct {
	capacity  domain.Capacity
	createErr error
	cancel    context.CancelFunc
	owner     uuid.UUID
	key       string
}

func (r *creationRepository) Summary(context.Context, uuid.UUID) (domain.Capacity, error) {
	return r.capacity, nil
}
func (r *creationRepository) Create(_ context.Context, owner uuid.UUID, name, personality, key string) (domain.Avatar, error) {
	r.owner, r.key = owner, key
	if r.cancel != nil {
		r.cancel()
	}
	return domain.Avatar{Name: name, Personality: personality}, r.createErr
}

type conversionRunner struct {
	asset *converter.Asset
	err   error
	calls int
}

func (r *conversionRunner) Convert(context.Context, io.ReadSeeker, string) (*converter.Asset, error) {
	r.calls++
	return r.asset, r.err
}

type conversionStore struct {
	objects              map[string]bool
	uploadErr, deleteErr error
	cleanupErr           error
	cleanupRemaining     time.Duration
}

func (s *conversionStore) Upload(context.Context, io.ReadSeeker, string) (string, error) {
	if s.uploadErr != nil {
		return "", s.uploadErr
	}
	s.objects["converted.vrm"] = true
	return "converted.vrm", nil
}
func (s *conversionStore) Delete(ctx context.Context, key string) error {
	s.cleanupErr = ctx.Err()
	if deadline, ok := ctx.Deadline(); ok {
		s.cleanupRemaining = time.Until(deadline)
	}
	if s.deleteErr != nil {
		return s.deleteErr
	}
	delete(s.objects, key)
	return nil
}

// Build a small valid source so persistence tests exercise validation as well.
func avatarSource(t *testing.T) []byte {
	t.Helper()
	profile, err := os.ReadFile("../converter/avaturn_profile.json")
	if err != nil {
		t.Fatal(err)
	}
	var p struct {
		Bones map[string]string `json:"bones"`
	}
	if err := json.Unmarshal(profile, &p); err != nil {
		t.Fatal(err)
	}
	doc := &gltf.Document{Asset: gltf.Asset{Version: "2.0", Generator: "Avaturn.me test"}}
	skin := &gltf.Skin{}
	for _, name := range p.Bones {
		skin.Joints = append(skin.Joints, len(doc.Nodes))
		doc.Nodes = append(doc.Nodes, &gltf.Node{Name: name})
	}
	matrices := modeler.WriteInverseBindMatrices(doc, make([][4][4]float32, len(skin.Joints)))
	skin.InverseBindMatrices = &matrices
	doc.Skins = []*gltf.Skin{skin}
	position := modeler.WritePosition(doc, [][3]float32{{0, 0, 0}})
	delta := modeler.WritePosition(doc, [][3]float32{{1, 0, 0}})
	for _, name := range []string{"Head_Mesh", "Teeth_Mesh", "Tongue_Mesh"} {
		primitive := &gltf.Primitive{Attributes: gltf.PrimitiveAttributes{gltf.POSITION: position}}
		for range 5 {
			primitive.Targets = append(primitive.Targets, gltf.PrimitiveAttributes{gltf.POSITION: delta})
		}
		doc.Meshes = append(doc.Meshes, &gltf.Mesh{Name: name, Primitives: []*gltf.Primitive{primitive}, Extras: map[string]any{"targetNames": []string{"viseme_aa", "viseme_I", "viseme_U", "viseme_E", "viseme_O"}}})
	}
	var data bytes.Buffer
	encoder := gltf.NewEncoder(&data)
	encoder.AsBinary = true
	if err := encoder.Encode(doc); err != nil {
		t.Fatal(err)
	}
	if _, err := converter.Validate(bytes.NewReader(data.Bytes()), int64(data.Len()), false); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}

func TestAvatarCreationPersistence(t *testing.T) {
	source := avatarSource(t)
	dbErr := errors.New("database unavailable")
	for _, tc := range []struct {
		name                             string
		insertErr, uploadErr, cleanupErr error
		cancel                           bool
	}{
		{name: "success"},
		{name: "inventory filled during conversion", insertErr: apperror.New(apperror.CodeInventoryFull, "full")},
		{name: "database failure", insertErr: dbErr},
		{name: "request cancelled before cleanup", insertErr: context.Canceled, cancel: true},
		{name: "cleanup failure preserves database error", insertErr: dbErr, cleanupErr: errors.New("storage unavailable")},
		{name: "upload failure", uploadErr: errors.New("upload failed")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			repo := &creationRepository{createErr: tc.insertErr}
			if tc.cancel {
				repo.cancel = cancel
			}
			store := &conversionStore{objects: map[string]bool{}, uploadErr: tc.uploadErr, deleteErr: tc.cleanupErr}
			file, err := os.CreateTemp(t.TempDir(), "asset-*.vrm")
			if err != nil {
				t.Fatal(err)
			}
			runner := &conversionRunner{asset: &converter.Asset{File: file}}
			svc := &AvatarConversionService{repo: repo, store: store, runner: runner, config: config.AvatarConversionConfig{Enabled: true, MaxInputBytes: 50 << 20}}
			user := inventoryUser()
			avatar, err := svc.Create(ctx, user, bytes.NewReader(source), int64(len(source)), domain.CreateAvatarInput{Name: "Test", Personality: "Friendly"})
			switch {
			case tc.uploadErr != nil:
				if !apperror.IsCode(err, apperror.CodeAvatarPersistenceFailed) {
					t.Fatalf("upload error = %v", err)
				}
				if repo.key != "" {
					t.Fatal("failed upload created a row")
				}
			case tc.insertErr != nil:
				if !errors.Is(err, tc.insertErr) {
					t.Fatalf("insert error lost: %v", err)
				}
				if tc.cleanupErr == nil && len(store.objects) != 0 {
					t.Fatal("failed insert left an orphan VRM")
				}
				if store.cleanupErr != nil || store.cleanupRemaining <= 0 || store.cleanupRemaining > 5*time.Second {
					t.Fatalf("cleanup must use a live bounded context: %v, %v", store.cleanupErr, store.cleanupRemaining)
				}
			default:
				if err != nil || avatar.Name != "Test" || repo.owner != user.ID || repo.key != "converted.vrm" || !store.objects[repo.key] {
					t.Fatalf("creation = %+v, %v", avatar, err)
				}
			}
			if _, err := file.Stat(); !errors.Is(err, os.ErrClosed) {
				t.Fatal("converted file not closed")
			}
		})
	}
}

func TestAvatarCreationConverterErrors(t *testing.T) {
	source := avatarSource(t)
	for _, tc := range []struct {
		err  error
		code apperror.Code
	}{
		{converter.ErrUnavailable, apperror.CodeAvatarConverterUnavailable},
		{context.DeadlineExceeded, apperror.CodeAvatarConversionTimeout},
		{errors.New("export failed"), apperror.CodeAvatarConversionFailed},
	} {
		runner := &conversionRunner{err: tc.err}
		svc := &AvatarConversionService{repo: &creationRepository{}, runner: runner, config: config.AvatarConversionConfig{Enabled: true, MaxInputBytes: 50 << 20}}
		_, err := svc.Create(context.Background(), inventoryUser(), bytes.NewReader(source), int64(len(source)), domain.CreateAvatarInput{Name: "Test", Personality: "Friendly"})
		if !apperror.IsCode(err, tc.code) || !errors.Is(err, tc.err) {
			t.Fatalf("converter error = %v, want %s", err, tc.code)
		}
	}
}
