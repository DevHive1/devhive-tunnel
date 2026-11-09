// Package aigen is the built-in "AI" network designer: a rules-and-expertise
// engine that turns a few answers into a fully hardened network plan
// (no external API required, works offline on any device).
package aigen

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"devhive-tunnel/pkg/hub"
)

// Profile selects the security posture.
type Profile string

const (
	ProfilePrivate   Profile = "private"   // family / trusted circle
	ProfileTeam      Profile = "team"      // company / project group
	ProfileCommunity Profile = "community" // public community
	ProfileParanoid  Profile = "paranoid"  // maximum hardening
)

// Input is what the wizard asks.
type Input struct {
	Name        string  `json:"name"`
	Profile     Profile `json:"profile"`
	Members     int     `json:"members"`      // expected member count
	LANGames    bool    `json:"lan_games"`    // needs full subnet + broadcast
	FileSharing bool    `json:"file_sharing"` // large transfers
	Language    string  `json:"language"`     // en|ar
}

// Plan is the generated configuration.
type Plan struct {
	Name          string          `json:"name"`
	Settings      hub.NetSettings `json:"settings"`
	Password      string          `json:"suggested_password"`
	Rationale     []string        `json:"rationale"`
	Checklist     []string        `json:"checklist"`
	SubnetCIDR    string          `json:"subnet_cidr"`
	SecurityScore int             `json:"security_score"` // 0-100
}

// SuggestPassword creates a strong, typeable password (no ambiguous chars).
func SuggestPassword() string {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789"
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	var sb strings.Builder
	// 4 groups of 4 for typeability
	for g := 0; g < 4; g++ {
		if g > 0 {
			sb.WriteByte('-')
		}
		for i := 0; i < 4; i++ {
			sb.WriteByte(alphabet[r.Intn(len(alphabet))])
		}
	}
	return sb.String()
}

// subnetFor picks a collision-resistant /16 in the shared CGNAT-friendly space.
func subnetFor(name string) string {
	r := rand.New(rand.NewSource(time.Now().UnixNano() ^ int64(len(name))*7919))
	second := 72 + r.Intn(24) // 100.72.0.0 – 100.95.0.0
	return fmt.Sprintf("100.%d.0.0/16", second)
}

// Generate builds the plan from user answers.
func Generate(in Input) Plan {
	if in.Members <= 0 {
		in.Members = 8
	}
	if in.Profile == "" {
		in.Profile = ProfileTeam
	}
	ar := strings.EqualFold(in.Language, "ar")

	p := Plan{Name: in.Name, SubnetCIDR: subnetFor(in.Name), Password: SuggestPassword()}
	s := hub.NetSettings{
		AllowRelay:     true,
		SubnetCIDR:     p.SubnetCIDR,
		MaxPeers:       in.Members + in.Members/2 + 2,
		MaxUploadMbps:  0,
		RateJoinPerMin: 10,
	}
	add := func(en, arabic string) {
		if ar {
			p.Rationale = append(p.Rationale, arabic)
		} else {
			p.Rationale = append(p.Rationale, en)
		}
	}

	switch in.Profile {
	case ProfilePrivate:
		s.RequireInviteKey = true
		s.AllowAnonymous = false
		s.MaxPeers = maxInt(4, in.Members)
		p.SecurityScore = 78
		add("Invite-only access: nobody joins with the password alone.",
			"الدخول بالدعوة فقط: لا يمكن لأحد الانضمام بكلمة المرور وحدها.")
		add("Network is hidden from the public list.",
			"الشبكة مخفية من القائمة العامة.")
	case ProfileTeam:
		s.RequireInviteKey = true
		s.AllowAnonymous = false
		s.MaxUploadMbps = 25
		p.SecurityScore = 74
		add("Work profile: invite key enforced, relay capped at 25 Mbps.",
			"ملف العمل: مفتاح الدعوة إلزامي، وتمرير البيانات محدود بـ 25 ميجابت.")
		add("Anonymous discovery disabled — the name cannot be scanned.",
			"البحث العام معطل — لا يمكن العثور على الاسم عبر الفحص.")
	case ProfileCommunity:
		s.RequireInviteKey = true
		s.AllowAnonymous = true
		s.MaxPeers = maxInt(12, in.Members*2)
		s.RateJoinPerMin = 6
		p.SecurityScore = 66
		add("Community profile: public listing but invite keys still required.",
			"ملف المجتمع: ظهور عام لكن مفاتيح الدعوة ما زالت مطلوبة.")
		add("Join rate limited to 6/min to stop password guessing.",
			"معدل الانضمام محدود بـ 6/دقيقة لمنع تخمين كلمات المرور.")
	case ProfileParanoid:
		s.RequireInviteKey = true
		s.AllowAnonymous = false
		s.MaxPeers = maxInt(3, in.Members)
		s.MaxUploadMbps = 10
		s.RateJoinPerMin = 3
		p.SecurityScore = 92
		add("Paranoid: tiny member cap, hard relay limits, 3 joins/min.",
			"الحصن: حد أعضاء صغير، حدود تمرير صارمة، 3 محاولات انضمام بالدقيقة.")
		add("All traffic is end-to-end encrypted regardless of profile; keep keys offline.",
			"كل البيانات مشفرة من طرف إلى طرف دائمًا؛ احفظ المفاتيح بعيدًا عن الأجهزة.")
	}

	if in.LANGames {
		add("LAN mode enabled: virtual subnet reserved for game discovery and broadcasts.",
			"وضع الشبكة المحلية مفعّل: نطاق افتراضي مخصص لاكتشاف الألعاب والبث.")
		add("Desktops get a real TUN adapter; phones use the relay/pass-through mode.",
			"أجهزة الحاسوب تحصل على محول شبكة حقيقي؛ الهواتف تستخدم وضع التمرير.")
	}
	if in.FileSharing {
		if s.MaxUploadMbps == 0 {
			s.MaxUploadMbps = 50
		}
		add("File sharing tuned: 192 KiB encrypted chunks with checksum verify.",
			"مشاركة الملفات مهيأة: أجزاء مشفرة 192 كيلوبايت مع تحقق بالبصمة.")
	}

	// Universal hardening notes
	add("ChaCha20-Poly1305 + X25519 + Ed25519 by default; passwords hashed with Argon2id.",
		"تشفير ChaCha20-Poly1305 وX25519 وEd25519 افتراضيًا؛ كلمات المرور بتجزئة Argon2id.")
	add("Owner keeps the only dashboard key; rotate it after big changes.",
		"المالك يحتفظ بمفتاح اللوحة الوحيد؛ بدّله بعد أي تغيير كبير.")

	p.Settings = s
	if ar {
		p.Checklist = []string{
			"احفظ كلمة المرور ومفتاح الإدارة في مكان آمن",
			"أرسل دعوات فردية لكل عضو بدل مشاركة كلمة المرور",
			"راجع قائمة الأعضاء أسبوعيًا",
			"بدّل مفتاح الإدارة عند مغادرة أي عضو",
		}
	} else {
		p.Checklist = []string{
			"Store the password and admin key somewhere safe",
			"Send individual invites instead of sharing the password",
			"Review the member list weekly",
			"Rotate the admin key when any member leaves",
		}
	}
	return p
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
