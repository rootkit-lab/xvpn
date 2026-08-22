package store

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"gorm.io/gorm"
)

const (
	NetworkKindInfra  = "infra"
	NetworkKindUsers  = "users"
	NetworkKindCustom = "custom"

	NetworkSubjectUser       = "user"
	NetworkSubjectDevice     = "device"
	NetworkSubjectMeshServer = "mesh_server"

	NetworkMemberRole = "member"
	NetworkOperator   = "operator"

	NetworkRuleAllow = "allow"
	NetworkRuleDeny  = "deny"

	InfraCIDR      = "10.66.66.0/24"
	UsersCIDR      = "10.66.80.0/24"
	UsersPoolCIDR  = "10.66.80.0/20"
	ControlPlaneIP = "10.66.66.1"
	CodespaceVIP   = "10.66.66.254"
)

var (
	forbiddenNets = []string{"10.10.0.0/16", "10.136.0.0/16"}
)

// OverlayNetwork é uma faixa overlay no mesmo wg0 do hub (Fase 67).
type OverlayNetwork struct {
	ID     uint   `gorm:"primaryKey"`
	Slug   string `gorm:"uniqueIndex;not null"`
	Name   string `gorm:"not null"`
	Kind   string `gorm:"not null"`
	CIDR   string `gorm:"uniqueIndex;not null"`
	System bool   `gorm:"not null;default:false"`
	Exit   bool   `gorm:"not null;default:false"`
}

// NetworkMember liga user/device/mesh a uma rede (rota + FORWARD implícito).
type NetworkMember struct {
	ID          uint   `gorm:"primaryKey"`
	NetworkID   uint   `gorm:"uniqueIndex:idx_net_member;not null"`
	SubjectKind string `gorm:"uniqueIndex:idx_net_member;not null"`
	SubjectID   uint   `gorm:"uniqueIndex:idx_net_member;not null"`
	Role        string `gorm:"not null;default:member"`
}

// NetworkRule é o FORWARD explícito entre duas redes.
type NetworkRule struct {
	ID           uint   `gorm:"primaryKey"`
	Slug         string `gorm:"uniqueIndex;not null"`
	SrcNetworkID uint   `gorm:"not null;index"`
	DstNetworkID uint   `gorm:"not null;index"`
	Action       string `gorm:"not null"`
	Proto        string `gorm:"not null;default:any"`
	Ports        string `gorm:"not null;default:''"`
	System       bool   `gorm:"not null;default:false"`
}

func ValidNetworkKind(k string) bool {
	switch k {
	case NetworkKindInfra, NetworkKindUsers, NetworkKindCustom:
		return true
	}
	return false
}

func ValidNetworkSubject(k string) bool {
	switch k {
	case NetworkSubjectUser, NetworkSubjectDevice, NetworkSubjectMeshServer:
		return true
	}
	return false
}

func NetworkByKind(db *gorm.DB, kind string) (OverlayNetwork, error) {
	var n OverlayNetwork
	err := db.Where("kind = ?", kind).First(&n).Error
	return n, err
}

func NetworkBySlug(db *gorm.DB, slug string) (OverlayNetwork, error) {
	var n OverlayNetwork
	err := db.Where("slug = ?", slug).First(&n).Error
	return n, err
}

func CIDRContainsIP(cidr, addr string) bool {
	_, n, err := net.ParseCIDR(cidr)
	if err != nil {
		return false
	}
	ip := parseHostIP(addr)
	return ip != nil && n.Contains(ip)
}

func parseHostIP(addr string) net.IP {
	if ip, _, err := net.ParseCIDR(addr); err == nil {
		return ip
	}
	return net.ParseIP(strings.TrimSpace(addr))
}

func cidrsOverlap(a, b string) bool {
	_, na, errA := net.ParseCIDR(a)
	_, nb, errB := net.ParseCIDR(b)
	if errA != nil || errB != nil {
		return false
	}
	return na.Contains(nb.IP) || nb.Contains(na.IP)
}

func cidrContained(inner, outer string) bool {
	_, in, errI := net.ParseCIDR(inner)
	_, out, errO := net.ParseCIDR(outer)
	if errI != nil || errO != nil {
		return false
	}
	onesI, bitsI := in.Mask.Size()
	onesO, bitsO := out.Mask.Size()
	if bitsI != bitsO || onesI < onesO {
		return false
	}
	return out.Contains(in.IP)
}

// ValidateOverlayCIDR recusa 10.10/10.136, overlap e custom fora do pool.
func ValidateOverlayCIDR(kind, cidr string, existing []OverlayNetwork) error {
	ip, n, err := net.ParseCIDR(strings.TrimSpace(cidr))
	if err != nil || ip.To4() == nil {
		return fmt.Errorf("CIDR IPv4 inválido")
	}
	if !n.IP.Equal(ip.Mask(n.Mask)) {
		return fmt.Errorf("CIDR deve ser o endereço de rede")
	}
	ones, bits := n.Mask.Size()
	if bits != 32 || ones < 24 || ones > 28 {
		return fmt.Errorf("prefixo deve ser /24–/28")
	}
	for _, bad := range forbiddenNets {
		if cidrsOverlap(cidr, bad) {
			return fmt.Errorf("CIDR colide com rede proibida")
		}
	}
	switch kind {
	case NetworkKindInfra:
		if cidr != InfraCIDR {
			return fmt.Errorf("infra é %s", InfraCIDR)
		}
	case NetworkKindUsers:
		if cidr != UsersCIDR {
			return fmt.Errorf("users é %s", UsersCIDR)
		}
	case NetworkKindCustom:
		if !cidrContained(cidr, UsersPoolCIDR) {
			return fmt.Errorf("custom deve estar em %s", UsersPoolCIDR)
		}
		if cidrsOverlap(cidr, UsersCIDR) {
			return fmt.Errorf("custom não pode sobrepor users")
		}
		if cidrsOverlap(cidr, InfraCIDR) {
			return fmt.Errorf("custom não pode sobrepor infra")
		}
	default:
		return fmt.Errorf("kind inválido")
	}
	for _, o := range existing {
		if cidrsOverlap(cidr, o.CIDR) {
			return fmt.Errorf("CIDR sobrepõe %s", o.Slug)
		}
	}
	return nil
}

func NextCustomCIDR(existing []OverlayNetwork) (string, error) {
	used := make(map[string]struct{}, len(existing))
	for _, n := range existing {
		used[n.CIDR] = struct{}{}
	}
	for i := 81; i <= 95; i++ {
		c := fmt.Sprintf("10.66.%d.0/24", i)
		if _, ok := used[c]; ok {
			continue
		}
		if err := ValidateOverlayCIDR(NetworkKindCustom, c, existing); err != nil {
			continue
		}
		return c, nil
	}
	return "", fmt.Errorf("pool %s esgotado", UsersPoolCIDR)
}

func ParseRulePorts(raw string) ([]int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var out []int
	for _, p := range strings.Split(raw, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil || n < 1 || n > 65535 {
			return nil, fmt.Errorf("porta inválida")
		}
		out = append(out, n)
	}
	return out, nil
}

func ValidRuleProto(p string) bool {
	switch strings.ToLower(strings.TrimSpace(p)) {
	case "tcp", "udp", "any":
		return true
	}
	return false
}

// SeedOverlayNetworks cria infra/users + regras sistema e move devices
// que não estão na rede certa (sem deixar /32 de usuário em 10.66.66.0/24).
func SeedOverlayNetworks(db *gorm.DB) error {
	if err := ensureSystemNetworks(db); err != nil {
		return err
	}
	if err := ensureSystemRules(db); err != nil {
		return err
	}
	return RehomeDevices(db)
}

func ensureSystemNetworks(db *gorm.DB) error {
	specs := []OverlayNetwork{
		{Slug: "infra", Name: "Infraestrutura", Kind: NetworkKindInfra, CIDR: InfraCIDR, System: true, Exit: false},
		{Slug: "users", Name: "Usuários", Kind: NetworkKindUsers, CIDR: UsersCIDR, System: true, Exit: true},
	}
	for _, spec := range specs {
		var n OverlayNetwork
		err := db.Where("slug = ?", spec.Slug).First(&n).Error
		if err == nil {
			n.Kind = spec.Kind
			n.CIDR = spec.CIDR
			n.System = true
			n.Exit = spec.Exit
			n.Name = spec.Name
			if err := db.Save(&n).Error; err != nil {
				return err
			}
			continue
		}
		if err != gorm.ErrRecordNotFound {
			return err
		}
		if err := db.Create(&spec).Error; err != nil {
			return err
		}
	}
	return nil
}

func ensureSystemRules(db *gorm.DB) error {
	users, err := NetworkByKind(db, NetworkKindUsers)
	if err != nil {
		return err
	}
	infra, err := NetworkByKind(db, NetworkKindInfra)
	if err != nil {
		return err
	}
	rules := []NetworkRule{
		{Slug: "corp", SrcNetworkID: users.ID, DstNetworkID: infra.ID, Action: NetworkRuleAllow, Proto: "tcp", Ports: "443,53", System: true},
		{Slug: "corp-dns", SrcNetworkID: users.ID, DstNetworkID: infra.ID, Action: NetworkRuleAllow, Proto: "udp", Ports: "53", System: true},
		{Slug: "samba", SrcNetworkID: users.ID, DstNetworkID: infra.ID, Action: NetworkRuleAllow, Proto: "tcp", Ports: "445", System: true},
	}
	for _, spec := range rules {
		var row NetworkRule
		err := db.Where("slug = ?", spec.Slug).First(&row).Error
		if err == nil {
			continue
		}
		if err != gorm.ErrRecordNotFound {
			return err
		}
		if err := db.Create(&spec).Error; err != nil {
			return err
		}
	}
	return nil
}

// RehomeDevices põe mesh em infra e o resto em users. Sem IP legado.
func RehomeDevices(db *gorm.DB) error {
	infra, err := NetworkByKind(db, NetworkKindInfra)
	if err != nil {
		return err
	}
	users, err := NetworkByKind(db, NetworkKindUsers)
	if err != nil {
		return err
	}
	var mesh []MeshServer
	if err := db.Find(&mesh).Error; err != nil {
		return err
	}
	meshDev := map[uint]struct{}{}
	for _, s := range mesh {
		if s.DeviceID != nil {
			meshDev[*s.DeviceID] = struct{}{}
		}
	}
	var devices []Device
	if err := db.Find(&devices).Error; err != nil {
		return err
	}
	used := make([]string, 0, len(devices))
	for _, d := range devices {
		used = append(used, d.AllowedIP)
	}
	for i := range devices {
		target := users
		if _, ok := meshDev[devices[i].ID]; ok {
			target = infra
		}
		inTarget := CIDRContainsIP(target.CIDR, devices[i].AllowedIP)
		if devices[i].NetworkID == target.ID && inTarget {
			if err := EnsureDeviceMember(db, target.ID, devices[i].ID, devices[i].UserID); err != nil {
				return err
			}
			continue
		}
		oldIP := devices[i].AllowedIP
		if !inTarget {
			ip, err := allocateIn(target.CIDR, used)
			if err != nil {
				return err
			}
			used = append(used, ip)
			devices[i].AllowedIP = ip
		}
		devices[i].NetworkID = target.ID
		if err := db.Save(&devices[i]).Error; err != nil {
			return err
		}
		if _, isMesh := meshDev[devices[i].ID]; isMesh && devices[i].AllowedIP != oldIP {
			wgIP := strings.TrimSuffix(devices[i].AllowedIP, "/32")
			if err := db.Model(&MeshServer{}).Where("device_id = ?", devices[i].ID).Update("wg_ip", wgIP).Error; err != nil {
				return err
			}
		}
		if err := EnsureDeviceMember(db, target.ID, devices[i].ID, devices[i].UserID); err != nil {
			return err
		}
	}
	return nil
}

func allocateIn(cidr string, used []string) (string, error) {
	// import cycle: store cannot import wireguard. Inline the same skip
	// (.0, first host, broadcast, VIP).
	return allocateOverlayIP(cidr, used)
}

func EnsureDeviceMember(db *gorm.DB, networkID, deviceID, userID uint) error {
	for _, pair := range []struct {
		kind string
		id   uint
	}{
		{NetworkSubjectDevice, deviceID},
		{NetworkSubjectUser, userID},
	} {
		var m NetworkMember
		err := db.Where("network_id = ? AND subject_kind = ? AND subject_id = ?", networkID, pair.kind, pair.id).First(&m).Error
		if err == nil {
			continue
		}
		if err != gorm.ErrRecordNotFound {
			return err
		}
		if err := db.Create(&NetworkMember{
			NetworkID: networkID, SubjectKind: pair.kind, SubjectID: pair.id, Role: NetworkMemberRole,
		}).Error; err != nil {
			return err
		}
	}
	return nil
}

// OverlayContainsIP is true if ip belongs to any overlay CIDR.
func OverlayContainsIP(db *gorm.DB, ip net.IP) bool {
	if ip == nil {
		return false
	}
	var nets []OverlayNetwork
	if db.Find(&nets).Error != nil {
		return false
	}
	for _, n := range nets {
		if CIDRContainsIP(n.CIDR, ip.String()) {
			return true
		}
	}
	return false
}

// ClientAllowedIPs is the peer AllowedIPs string. Mesh = infra CIDR.
// User home with exit = full tunnel. No 10.66.66.0/24 default for users.
func ClientAllowedIPs(db *gorm.DB, device Device) (string, error) {
	if device.NetworkID == 0 {
		return "", fmt.Errorf("dispositivo sem rede")
	}
	var home OverlayNetwork
	if err := db.First(&home, device.NetworkID).Error; err != nil {
		return "", err
	}
	if home.Kind == NetworkKindInfra {
		return home.CIDR, nil
	}
	if home.Exit {
		return "0.0.0.0/0, ::/0", nil
	}
	cidrs := map[string]struct{}{home.CIDR: {}}
	if err := addMemberCIDRs(db, device, cidrs); err != nil {
		return "", err
	}
	if err := addRuleDestCIDRs(db, home.ID, cidrs); err != nil {
		return "", err
	}
	out := make([]string, 0, len(cidrs))
	for c := range cidrs {
		out = append(out, c)
	}
	return strings.Join(out, ", "), nil
}

func addMemberCIDRs(db *gorm.DB, device Device, cidrs map[string]struct{}) error {
	var meshIDs []uint
	if err := db.Model(&MeshServer{}).Where("device_id = ?", device.ID).Pluck("id", &meshIDs).Error; err != nil {
		return err
	}
	q := db.Where(
		"(subject_kind = ? AND subject_id = ?) OR (subject_kind = ? AND subject_id = ?)",
		NetworkSubjectDevice, device.ID, NetworkSubjectUser, device.UserID,
	)
	if len(meshIDs) > 0 {
		q = q.Or("subject_kind = ? AND subject_id IN ?", NetworkSubjectMeshServer, meshIDs)
	}
	var members []NetworkMember
	if err := q.Find(&members).Error; err != nil {
		return err
	}
	for _, m := range members {
		var n OverlayNetwork
		if err := db.First(&n, m.NetworkID).Error; err != nil {
			return err
		}
		cidrs[n.CIDR] = struct{}{}
	}
	return nil
}

func addRuleDestCIDRs(db *gorm.DB, homeID uint, cidrs map[string]struct{}) error {
	var rules []NetworkRule
	if err := db.Where("src_network_id = ? AND action = ?", homeID, NetworkRuleAllow).Find(&rules).Error; err != nil {
		return err
	}
	for _, r := range rules {
		var n OverlayNetwork
		if err := db.First(&n, r.DstNetworkID).Error; err != nil {
			return err
		}
		cidrs[n.CIDR] = struct{}{}
	}
	return nil
}

// ForwardAllowed is the hub policy: same net ok; otherwise only an
// explicit allow rule. Membership is route (AllowedIPs), not FORWARD.
func ForwardAllowed(rules []NetworkRule, _ [][2]uint, srcID, dstID uint, proto string, port int) bool {
	if srcID == 0 || dstID == 0 {
		return false
	}
	if srcID == dstID {
		return true
	}
	proto = strings.ToLower(proto)
	for _, r := range rules {
		if r.SrcNetworkID != srcID || r.DstNetworkID != dstID || r.Action != NetworkRuleAllow {
			continue
		}
		rp := strings.ToLower(r.Proto)
		if rp != "any" && rp != proto {
			continue
		}
		ports, err := ParseRulePorts(r.Ports)
		if err != nil {
			continue
		}
		if len(ports) == 0 {
			return true
		}
		for _, p := range ports {
			if p == port {
				return true
			}
		}
	}
	return false
}

func MembershipPairs(db *gorm.DB) ([][2]uint, error) {
	var members []NetworkMember
	if err := db.Find(&members).Error; err != nil {
		return nil, err
	}
	var devices []Device
	if err := db.Find(&devices).Error; err != nil {
		return nil, err
	}
	var mesh []MeshServer
	if err := db.Find(&mesh).Error; err != nil {
		return nil, err
	}
	homeByDevice := map[uint]uint{}
	homeByUser := map[uint][]uint{}
	homeByMesh := map[uint]uint{}
	for _, d := range devices {
		if d.NetworkID == 0 {
			continue
		}
		homeByDevice[d.ID] = d.NetworkID
		homeByUser[d.UserID] = append(homeByUser[d.UserID], d.NetworkID)
	}
	for _, s := range mesh {
		if s.DeviceID != nil {
			homeByMesh[s.ID] = homeByDevice[*s.DeviceID]
		}
	}
	seen := map[[2]uint]struct{}{}
	var out [][2]uint
	add := func(src, dst uint) {
		if src == 0 || dst == 0 || src == dst {
			return
		}
		p := [2]uint{src, dst}
		if _, ok := seen[p]; ok {
			return
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	for _, m := range members {
		switch m.SubjectKind {
		case NetworkSubjectDevice:
			add(homeByDevice[m.SubjectID], m.NetworkID)
		case NetworkSubjectUser:
			for _, hid := range homeByUser[m.SubjectID] {
				add(hid, m.NetworkID)
			}
		case NetworkSubjectMeshServer:
			add(homeByMesh[m.SubjectID], m.NetworkID)
		}
	}
	return out, nil
}
