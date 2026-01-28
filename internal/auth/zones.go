package auth

import (
	"context"
	"net"
	"sync"
)

type Zone struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Subnet      string            `json:"subnet,omitempty"`
	Campus      string            `json:"campus,omitempty"`
	Building    string            `json:"building,omitempty"`
	Floor       string            `json:"floor,omitempty"`
	AccessLevel int               `json:"access_level"`
	AllowedFrom []string          `json:"allowed_from,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

type ZonePolicy struct {
	ZoneID          string           `json:"zone_id"`
	RequireAuth     bool             `json:"require_auth"`
	AllowedRoles    []string         `json:"allowed_roles"`
	AllowedUsers    []string         `json:"allowed_users"`
	DeniedUsers     []string         `json:"denied_users"`
	RequireZoneAuth bool             `json:"require_zone_auth"`
	TimeRestriction *TimeRestriction `json:"time_restriction,omitempty"`
}

type TimeRestriction struct {
	AllowedDays []int  `json:"allowed_days"`
	StartHour   int    `json:"start_hour"`
	EndHour     int    `json:"end_hour"`
	Timezone    string `json:"timezone"`
}

type ZoneManager struct {
	zones    map[string]*Zone
	policies map[string]*ZonePolicy
	subnets  map[string]*net.IPNet
	mu       sync.RWMutex
}

func NewZoneManager() *ZoneManager {
	return &ZoneManager{
		zones:    make(map[string]*Zone),
		policies: make(map[string]*ZonePolicy),
		subnets:  make(map[string]*net.IPNet),
	}
}

func (zm *ZoneManager) RegisterZone(zone *Zone) error {
	zm.mu.Lock()
	defer zm.mu.Unlock()

	zm.zones[zone.ID] = zone

	if zone.Subnet != "" {
		_, ipNet, err := net.ParseCIDR(zone.Subnet)
		if err != nil {
			return err
		}
		zm.subnets[zone.ID] = ipNet
	}
	return nil
}

func (zm *ZoneManager) SetPolicy(policy *ZonePolicy) {
	zm.mu.Lock()
	defer zm.mu.Unlock()
	zm.policies[policy.ZoneID] = policy
}

func (zm *ZoneManager) GetZone(zoneID string) (*Zone, bool) {
	zm.mu.RLock()
	defer zm.mu.RUnlock()
	zone, ok := zm.zones[zoneID]
	return zone, ok
}

func (zm *ZoneManager) GetZoneForIP(ip string) *Zone {
	zm.mu.RLock()
	defer zm.mu.RUnlock()

	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return nil
	}

	for zoneID, subnet := range zm.subnets {
		if subnet.Contains(parsedIP) {
			if zone, ok := zm.zones[zoneID]; ok {
				return zone
			}
		}
	}
	return nil
}

func (zm *ZoneManager) CheckAccess(ctx context.Context, claims *Claims, targetZone string, clientIP string) error {
	zm.mu.RLock()
	defer zm.mu.RUnlock()

	zone, ok := zm.zones[targetZone]
	if !ok {
		return ErrInvalidZone
	}

	policy, hasPolicy := zm.policies[targetZone]
	if !hasPolicy {
		if zm.userHasZone(claims, targetZone) {
			return nil
		}
		return ErrInvalidZone
	}

	for _, deniedID := range policy.DeniedUsers {
		if claims.Subject == deniedID {
			return ErrInvalidZone
		}
	}

	for _, allowedID := range policy.AllowedUsers {
		if claims.Subject == allowedID {
			return nil
		}
	}

	if len(policy.AllowedRoles) > 0 {
		roleAllowed := false
		for _, role := range policy.AllowedRoles {
			if claims.Role == role {
				roleAllowed = true
				break
			}
		}
		if !roleAllowed {
			return ErrInvalidZone
		}
	}

	if policy.RequireZoneAuth {
		clientZone := zm.GetZoneForIP(clientIP)
		if clientZone == nil || clientZone.ID != targetZone {
			if !zm.isFromAllowedZone(zone, clientZone) {
				return ErrInvalidZone
			}
		}
	}

	if zone.AccessLevel > 0 && !zm.userHasZone(claims, targetZone) {
		return ErrInvalidZone
	}

	return nil
}

func (zm *ZoneManager) userHasZone(claims *Claims, targetZone string) bool {
	for _, z := range claims.Zones {
		if z == targetZone || z == "*" {
			return true
		}
	}
	return claims.Zone == targetZone
}

func (zm *ZoneManager) isFromAllowedZone(targetZone, clientZone *Zone) bool {
	if clientZone == nil {
		return false
	}
	for _, allowed := range targetZone.AllowedFrom {
		if allowed == clientZone.ID || allowed == "*" {
			return true
		}
	}
	return false
}

func (zm *ZoneManager) ListZones() []*Zone {
	zm.mu.RLock()
	defer zm.mu.RUnlock()

	zones := make([]*Zone, 0, len(zm.zones))
	for _, zone := range zm.zones {
		zones = append(zones, zone)
	}
	return zones
}
