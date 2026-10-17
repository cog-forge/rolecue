package converter

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/qmuntal/gltf"
	"github.com/qmuntal/gltf/modeler"
)

var ErrUnsupported = errors.New("unsupported avatar asset")

type Capabilities struct {
	LipSync    bool   `json:"lip_sync"`
	VRMVersion string `json:"vrm_version"`
}

var vowels = map[string]string{"aa": "viseme_aa", "ih": "viseme_I", "ou": "viseme_U", "ee": "viseme_E", "oh": "viseme_O"}
var requiredBones = []string{"hips", "spine", "head", "leftUpperLeg", "leftLowerLeg", "leftFoot", "rightUpperLeg", "rightLowerLeg", "rightFoot", "leftUpperArm", "leftLowerArm", "leftHand", "rightUpperArm", "rightLowerArm", "rightHand"}

var mouthMeshes = []string{"Head_Mesh", "Teeth_Mesh", "Tongue_Mesh"}

type vrmExtension struct {
	SpecVersion string `json:"specVersion"`
	Humanoid    struct {
		HumanBones map[string]struct {
			Node *int `json:"node"`
		} `json:"humanBones"`
	} `json:"humanoid"`
	Expressions struct {
		Preset map[string]struct {
			Binds []struct {
				Node   *int    `json:"node"`
				Index  *int    `json:"index"`
				Weight float64 `json:"weight"`
			} `json:"morphTargetBinds"`
		} `json:"preset"`
	} `json:"expressions"`
}

// Validate uses gltf's decoder without a filesystem, so uploads cannot read local files.
// Container lengths are checked first to bound the decoder's binary allocation.
func Validate(r io.ReadSeeker, size int64, output bool) (Capabilities, error) {
	fail := func(reason string) (Capabilities, error) {
		return Capabilities{}, fmt.Errorf("%w: %s", ErrUnsupported, reason)
	}
	if size < 28 || size > 100<<20 {
		return fail("invalid GLB size")
	}
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return fail("unreadable GLB")
	}
	var header [20]byte
	if _, err := io.ReadFull(r, header[:]); err != nil || string(header[:4]) != "glTF" || binary.LittleEndian.Uint32(header[4:8]) != 2 || int64(binary.LittleEndian.Uint32(header[8:12])) != size || string(header[16:20]) != "JSON" {
		return fail("invalid GLB container")
	}
	jsonLength := int64(binary.LittleEndian.Uint32(header[12:16]))
	if jsonLength <= 0 || jsonLength > 4<<20 || jsonLength%4 != 0 || jsonLength > size-28 {
		return fail("invalid JSON chunk")
	}
	if _, err := r.Seek(20+jsonLength, io.SeekStart); err != nil {
		return fail("invalid chunk offset")
	}
	var chunk [8]byte
	if _, err := io.ReadFull(r, chunk[:]); err != nil || string(chunk[4:]) != "BIN\x00" || int64(binary.LittleEndian.Uint32(chunk[:4])) != size-28-jsonLength {
		return fail("invalid binary chunk")
	}
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return fail("unreadable GLB")
	}
	var doc gltf.Document
	if err := gltf.NewDecoder(io.LimitReader(r, size)).Decode(&doc); err != nil {
		return fail("malformed glTF")
	}
	if doc.Asset.Version != "2.0" || len(doc.Nodes) > 2000 || len(doc.Meshes) == 0 || len(doc.Meshes) > 200 || len(doc.Skins) != 1 || len(doc.Accessors) > 10000 || len(doc.Buffers) != 1 {
		return fail("unsupported scene complexity")
	}
	for _, b := range doc.Buffers {
		if b == nil || b.URI != "" || b.ByteLength <= 0 || b.ByteLength > len(b.Data) {
			return fail("external or invalid buffer")
		}
	}
	for _, v := range doc.BufferViews {
		if _, err := modeler.ReadBufferView(&doc, v); err != nil {
			return fail("invalid buffer view")
		}
	}
	total := 0
	for _, a := range doc.Accessors {
		if a == nil || a.Count <= 0 || a.Count > 1000000 {
			return fail("invalid accessor count")
		}
		total += a.Count
		if total > 12000000 {
			return fail("accessor complexity exceeded")
		}
		if _, err := modeler.ReadAccessor(&doc, a, nil); err != nil {
			return fail("invalid accessor data")
		}
	}
	for _, image := range doc.Images {
		if image == nil || image.URI != "" || image.BufferView == nil || !valid(*image.BufferView, len(doc.BufferViews)) {
			return fail("external or missing image")
		}
	}
	for _, extension := range doc.ExtensionsRequired {
		if extension != "VRMC_vrm" && extension != "KHR_materials_ior" {
			return fail("unsupported required extension")
		}
	}
	for _, skin := range doc.Skins {
		if skin == nil || len(skin.Joints) < 15 || len(skin.Joints) > 200 {
			return fail("invalid humanoid skin")
		}
		for _, index := range skin.Joints {
			if !valid(index, len(doc.Nodes)) {
				return fail("invalid joint")
			}
		}
		if skin.InverseBindMatrices == nil || !valid(*skin.InverseBindMatrices, len(doc.Accessors)) {
			return fail("missing skin matrices")
		}
	}
	seen := make([]uint8, len(doc.Nodes))
	parents := make([]int, len(doc.Nodes))
	var visit func(int) bool
	visit = func(i int) bool {
		if !valid(i, len(doc.Nodes)) || doc.Nodes[i] == nil || seen[i] == 1 {
			return false
		}
		if seen[i] == 2 {
			return true
		}
		seen[i] = 1
		for _, child := range doc.Nodes[i].Children {
			if !visit(child) {
				return false
			}
		}
		seen[i] = 2
		return true
	}
	for i, n := range doc.Nodes {
		if !visit(i) {
			return fail("invalid node hierarchy")
		}
		if n.Mesh != nil && !valid(*n.Mesh, len(doc.Meshes)) || n.Skin != nil && !valid(*n.Skin, len(doc.Skins)) {
			return fail("invalid node reference")
		}
		for _, child := range n.Children {
			parents[child]++
			if parents[child] > 1 {
				return fail("node has multiple parents")
			}
		}
	}
	for _, mesh := range doc.Meshes {
		if mesh == nil || len(mesh.Primitives) == 0 {
			return fail("empty mesh")
		}
		for _, p := range mesh.Primitives {
			if p == nil {
				return fail("empty primitive")
			}
			for _, index := range p.Attributes {
				if !valid(index, len(doc.Accessors)) {
					return fail("invalid attribute")
				}
			}
			position, ok := p.Attributes[gltf.POSITION]
			if !ok {
				return fail("missing mesh positions")
			}
			if p.Indices != nil && !valid(*p.Indices, len(doc.Accessors)) || p.Material != nil && !valid(*p.Material, len(doc.Materials)) {
				return fail("invalid primitive reference")
			}
			for _, target := range p.Targets {
				for _, index := range target {
					if !valid(index, len(doc.Accessors)) {
						return fail("invalid morph reference")
					}
					if doc.Accessors[index].Count != doc.Accessors[position].Count {
						return fail("morph vertex count mismatch")
					}
				}
			}
		}
	}
	if !output {
		if !strings.HasPrefix(doc.Asset.Generator, "Avaturn.me") {
			return fail("Avaturn export required")
		}
		var profile struct {
			Bones map[string]string `json:"bones"`
		}
		if json.Unmarshal(profileJSON, &profile) != nil {
			return fail("invalid converter profile")
		}
		boneNames := make(map[string]bool)
		for _, joint := range doc.Skins[0].Joints {
			boneNames[doc.Nodes[joint].Name] = true
		}
		for _, bone := range profile.Bones {
			if !boneNames[bone] {
				return fail("unsupported Avaturn skeleton")
			}
		}
		for _, name := range vowels {
			found := make(map[string]bool)
			for _, mesh := range doc.Meshes {
				var extra struct {
					Names []string `json:"targetNames"`
				}
				data, _ := json.Marshal(mesh.Extras)
				_ = json.Unmarshal(data, &extra)
				for index, target := range extra.Names {
					if target == name && moving(&doc, mesh, index) {
						found[mesh.Name] = true
					}
				}
			}
			for _, meshName := range mouthMeshes {
				if !found[meshName] {
					return fail("missing lip-sync deformation")
				}
			}
		}
		return Capabilities{LipSync: true}, nil
	}
	var extension vrmExtension
	data, err := json.Marshal(doc.Extensions["VRMC_vrm"])
	if err != nil || json.Unmarshal(data, &extension) != nil || extension.SpecVersion != "1.0" {
		return fail("VRM 1.0 required")
	}
	for _, name := range requiredBones {
		bone := extension.Humanoid.HumanBones[name]
		if bone.Node == nil || !valid(*bone.Node, len(doc.Nodes)) {
			return fail("missing humanoid bone")
		}
		isJoint := false
		for _, joint := range doc.Skins[0].Joints {
			if joint == *bone.Node {
				isJoint = true
				break
			}
		}
		if !isJoint {
			return fail("humanoid bone is not a skin joint")
		}
	}
	for name := range vowels {
		expression, ok := extension.Expressions.Preset[name]
		if !ok || len(expression.Binds) == 0 {
			return fail("missing vowel expression")
		}
		movingBindings := make(map[string]bool)
		for _, bind := range expression.Binds {
			if bind.Node == nil || bind.Index == nil || !valid(*bind.Node, len(doc.Nodes)) || bind.Weight <= 0 || bind.Weight > 1 {
				return fail("invalid expression binding")
			}
			node := doc.Nodes[*bind.Node]
			if node.Mesh == nil || !moving(&doc, doc.Meshes[*node.Mesh], *bind.Index) {
				return fail("empty expression deformation")
			}
			movingBindings[node.Name] = true
		}
		for _, meshName := range mouthMeshes {
			if !movingBindings[meshName] {
				return fail("incomplete mouth expression")
			}
		}
	}
	return Capabilities{LipSync: true, VRMVersion: "1.0"}, nil
}

func valid(index, length int) bool { return index >= 0 && index < length }

func moving(doc *gltf.Document, mesh *gltf.Mesh, index int) bool {
	changed := false
	for _, p := range mesh.Primitives {
		if !valid(index, len(p.Targets)) {
			return false
		}
		accessor, ok := p.Targets[index][gltf.POSITION]
		if !ok || !valid(accessor, len(doc.Accessors)) {
			return false
		}
		positions, err := modeler.ReadPosition(doc, doc.Accessors[accessor], nil)
		if err != nil {
			return false
		}
		for _, v := range positions {
			for _, x := range v {
				if math.IsNaN(float64(x)) || math.IsInf(float64(x), 0) {
					return false
				}
				if math.Abs(float64(x)) > 1e-8 {
					changed = true
				}
			}
		}
	}
	return changed
}
