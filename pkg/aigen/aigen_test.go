package aigen

import (
	"strings"
	"testing"
)

func TestProfiles(t *testing.T) {
	cases := []struct {
		p        Profile
		invite   bool
		anon     bool
		minScore int
	}{
		{ProfilePrivate, true, false, 70},
		{ProfileTeam, true, false, 70},
		{ProfileCommunity, true, true, 60},
		{ProfileParanoid, true, false, 90},
	}
	for _, c := range cases {
		plan := Generate(Input{Name: "hive", Profile: c.p, Members: 5})
		if plan.Settings.RequireInviteKey != c.invite {
			t.Errorf("%s: invite=%v want %v", c.p, plan.Settings.RequireInviteKey, c.invite)
		}
		if plan.Settings.AllowAnonymous != c.anon {
			t.Errorf("%s: anon=%v want %v", c.p, plan.Settings.AllowAnonymous, c.anon)
		}
		if plan.SecurityScore < c.minScore {
			t.Errorf("%s: score=%d want >=%d", c.p, plan.SecurityScore, c.minScore)
		}
	}
}

func TestPlanShape(t *testing.T) {
	plan := Generate(Input{Name: "games", Profile: ProfileTeam, Members: 10, LANGames: true})
	if !strings.HasPrefix(plan.SubnetCIDR, "100.") || !strings.HasSuffix(plan.SubnetCIDR, ".0.0/16") {
		t.Errorf("subnet %q not a 100.x/16", plan.SubnetCIDR)
	}
	if plan.Settings.SubnetCIDR != plan.SubnetCIDR {
		t.Errorf("settings subnet mismatch")
	}
	if plan.Settings.MaxPeers < 10 {
		t.Errorf("max peers %d too small for 10 members", plan.Settings.MaxPeers)
	}
	if len(plan.Password) < 15 || strings.Count(plan.Password, "-") != 3 {
		t.Errorf("password %q not 4x4 grouped", plan.Password)
	}
	if len(plan.Checklist) == 0 || len(plan.Rationale) == 0 {
		t.Errorf("empty rationale/checklist")
	}
}

func TestArabic(t *testing.T) {
	plan := Generate(Input{Name: "hive", Profile: ProfilePrivate, Members: 3, Language: "ar"})
	joined := strings.Join(plan.Rationale, " ")
	if !strings.ContainsAny(joined, "ابتثجحخدذرزسشصضطظعغفقكلمنهوي") {
		t.Errorf("expected Arabic rationale, got %q", plan.Rationale)
	}
}
