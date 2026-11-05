package converter

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/qmuntal/gltf"
	"github.com/qmuntal/gltf/modeler"
)

// limitSource builds a minimal Avaturn-shaped GLB. chain adds that many extra nodes linked parent to
// child, listed leaf first when reversed, and images adds that many embedded images.
func limitSource(t *testing.T, chain int, reversed bool, images int) *bytes.Reader {
	t.Helper()
	var profile struct {
		Bones map[string]string `json:"bones"`
	}
	if err := json.Unmarshal(profileJSON, &profile); err != nil {
		t.Fatal(err)
	}
	doc := &gltf.Document{Asset: gltf.Asset{Version: "2.0", Generator: "Avaturn.me test"}}
	skin := &gltf.Skin{}
	for _, name := range profile.Bones {
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
		doc.Meshes = append(doc.Meshes, &gltf.Mesh{Name: name, Primitives: []*gltf.Primitive{primitive}, Extras: map[string]any{
			"targetNames": []string{"viseme_aa", "viseme_I", "viseme_U", "viseme_E", "viseme_O"}}})
	}
	if chain > 0 {
		start := len(doc.Nodes)
		for i := range chain {
			node := &gltf.Node{Name: "chain"}
			switch {
			case reversed && i > 0:
				// Node i+1 is the parent of node i, so the walk meets the deepest leaf first.
				node.Children = []int{start + i - 1}
			case !reversed && i < chain-1:
				node.Children = []int{start + i + 1}
			}
			doc.Nodes = append(doc.Nodes, node)
		}
	}
	if images > 0 {
		view := modeler.WriteBufferView(doc, gltf.TargetNone, []byte{1, 2, 3, 4})
		for range images {
			doc.Images = append(doc.Images, &gltf.Image{BufferView: &view})
		}
	}
	var data bytes.Buffer
	encoder := gltf.NewEncoder(&data)
	encoder.AsBinary = true
	if err := encoder.Encode(doc); err != nil {
		t.Fatal(err)
	}
	return bytes.NewReader(data.Bytes())
}

func TestValidateAcceptsAShallowAvatar(t *testing.T) {
	source := limitSource(t, 10, false, 3)
	if _, err := Validate(source, source.Size(), false); err != nil {
		t.Fatalf("Validate() error = %v, want a shallow avatar accepted", err)
	}
}

func TestValidateRefusesDeeplyNestedNodesWhateverTheirOrder(t *testing.T) {
	for _, reversed := range []bool{false, true} {
		source := limitSource(t, maxNodeDepth+20, reversed, 0)
		if _, err := Validate(source, source.Size(), false); !errors.Is(err, ErrUnsupported) {
			t.Fatalf("reversed=%v: Validate() error = %v, want ErrUnsupported", reversed, err)
		}
	}
}

func TestValidateRefusesTooManyImages(t *testing.T) {
	source := limitSource(t, 0, false, maxImages+1)
	if _, err := Validate(source, source.Size(), false); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("Validate() error = %v, want ErrUnsupported", err)
	}
}
