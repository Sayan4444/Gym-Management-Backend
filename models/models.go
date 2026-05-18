package models

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/gosimple/slug"
	"github.com/lib/pq"
	"gorm.io/gorm"
)

type User struct {
	ID          uint           `gorm:"primarykey" json:"id"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	Name        string         `json:"name"`
	Email       string         `json:"email" gorm:"unique"`
	Phone       string         `json:"phone"`
	DOB         string         `json:"dob"`
	Gender      string         `json:"gender"`
	PhotoURL    string         `json:"photo_url"`
	BiometricID string         `json:"biometric_id"` // Simulated
	Role        string         `json:"role"`         // SuperAdmin, GymAdmin, Trainer, Member, User
	SocialMedia pq.StringArray `json:"social_media" gorm:"type:text[]"`
	// ["instagram", "facebook", "x"]
	Address               string         `json:"address"`
	EmergencyContactName  string         `json:"emergency_contact_name"`
	EmergencyContactPhone string         `json:"emergency_contact_phone"`
	BloodGroup            string         `json:"blood_group"`
	Height                *float64       `json:"height"`
	Weight                *float64       `json:"weight"`
	MedicalConditions     string         `json:"medical_conditions"`
	GymID                 *uint          `json:"gym_id" gorm:"index"`
	Gym                   *Gym           `json:"gym" gorm:"foreignKey:GymID"`
	Subscription          []Subscription `json:"subscription" gorm:"foreignKey:UserID"`
	UserAddon             []UserAddon    `json:"user_addon" gorm:"foreignKey:UserID"`
	TrainerID             *uint          `json:"trainer_id" gorm:"index"`
	Trainer               *User          `json:"trainer" gorm:"foreignKey:TrainerID"`
	Payments              []Payment      `json:"payments" gorm:"foreignKey:UserID"`
	WorkoutPlans          []WorkoutPlan  `json:"workout_plans" gorm:"foreignKey:MemberID"`
}

type Gym struct {
	ID              uint             `gorm:"primarykey" json:"id"`
	CreatedAt       time.Time        `json:"createdAt"`
	UpdatedAt       time.Time        `json:"updatedAt"`
	DeletedAt       gorm.DeletedAt   `gorm:"index" json:"-"`
	Name            string           `json:"name"`
	Slug            string           `json:"slug" gorm:"uniqueIndex"`
	Domain          string           `json:"domain" gorm:"uniqueIndex"`
	Address         string           `json:"address"`
	Whatsapp        string           `json:"whatsapp"`
	Email           string           `json:"email" gorm:"unique"`
	Phone           string           `json:"phone" gorm:"unique"`
	Users           []User           `gorm:"foreignKey:GymID" json:"users"`
	MembershipPlans []MembershipPlan `gorm:"foreignKey:GymID" json:"membership_plans"`
	Addons          []Addon          `gorm:"foreignKey:GymID" json:"addons"`
	GymIcon         string           `json:"gym_icon"`
}

// deal provided by the gym
// MembershipPlans available in the gym
type MembershipPlan struct {
	ID             uint           `gorm:"primarykey" json:"id"`
	CreatedAt      time.Time      `json:"createdAt"`
	UpdatedAt      time.Time      `json:"updatedAt"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
	GymID          uint           `json:"gym_id" gorm:"index"`
	Name           string         `json:"name"`
	Price          float64        `json:"price"`
	DurationMonths int            `json:"duration_months"`
	IsActive       bool           `json:"is_active" gorm:"default:true"` // is its a active plan
	PlanAddons     []PlanAddon    `json:"plan_addons" gorm:"foreignKey:PlanID"`
	PlanIcon       *string        `json:"plan_icon"`
}

// PlanAddon links an Addon to a MembershipPlan with a total count.
// Frequency is an integer: the number of times the member gets this addon throughout the plan.
// e.g., Frequency = 12 means 12 personal training sessions over the life of the plan.
type PlanAddon struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	PlanID    uint           `json:"plan_id" gorm:"index"`
	AddonID   uint           `json:"addon_id" gorm:"index"`
	Addon     *Addon         `json:"addon" gorm:"foreignKey:AddonID"`
	Frequency int            `json:"frequency"` // total count of addon usage included in the plan
}

// Subscription taken by the user.
// Status field stores only manual overrides: "Paused", "Cancelled".
// Time-based states (Active, Expired, Upcoming) are computed dynamically via CurrentStatus().
type Subscription struct {
	ID        uint            `gorm:"primarykey" json:"id"`
	CreatedAt time.Time       `json:"createdAt"`
	UpdatedAt time.Time       `json:"updatedAt"`
	DeletedAt gorm.DeletedAt  `gorm:"index" json:"-"`
	UserID    uint            `json:"user_id" gorm:"index"`
	PlanID    uint            `json:"plan_id" gorm:"index"`
	Plan      *MembershipPlan `json:"plan" gorm:"foreignKey:PlanID"`
	StartDate time.Time       `json:"start_date"`
	EndDate   time.Time       `json:"end_date"`
	Status    string          `json:"status" gorm:"default:''"` // Manual overrides only: "Paused", "Cancelled"
}

// CurrentStatus dynamically calculates the true status of the subscription.
// Manual overrides (Paused, Cancelled) take priority over time-based calculation.
func (s *Subscription) CurrentStatus() string {
	// 1. Check for manual overrides first
	if s.Status == "Paused" || s.Status == "Cancelled" {
		return s.Status
	}

	// 2. Calculate time-based status
	now := time.Now()

	if now.Before(s.StartDate) {
		return "Upcoming"
	}

	if now.After(s.EndDate) {
		return "Expired"
	}

	return "Active"
}

// MarshalJSON overrides the default JSON marshalling so that the API always
// returns the dynamically computed status instead of the raw DB field.
func (s Subscription) MarshalJSON() ([]byte, error) {
	type Alias Subscription
	return json.Marshal(&struct {
		Alias
		Status string `json:"status"`
	}{
		Alias:  Alias(s),
		Status: s.CurrentStatus(),
	})
}

// deal provided by the gym and taken by the user
// Addons available in the gym
type Addon struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	GymID     uint           `json:"gym_id" gorm:"index"`
	Name      string         `json:"name"`
	Price     float64        `json:"price"`
	IsActive  bool           `json:"is_active" gorm:"default:true"`
	Duration  int            `json:"duration"` // duration in minutes (0 = not set)
}

// UserAddon represents an addon purchased by a user.
// Status is computed dynamically from ScheduledAt and Addon.Duration.
type UserAddon struct {
	ID          uint           `gorm:"primarykey" json:"id"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	UserID      uint           `json:"user_id" gorm:"index"`
	AddonID     uint           `json:"addon_id" gorm:"index"`
	Addon       *Addon         `json:"addon" gorm:"foreignKey:AddonID"`
	PurchasedAt time.Time      `json:"purchased_at"`
	ScheduledAt *time.Time     `json:"scheduled_at,omitempty"` // optional scheduled date and time
}

// CurrentStatus dynamically calculates the status of a user addon session.
//   - "Purchased"   — no schedule set yet
//   - "Scheduled"   — session is in the future
//   - "In Progress" — session is happening right now
//   - "Completed"   — session has ended (ScheduledAt + Duration has passed)
func (ua *UserAddon) CurrentStatus() string {
	if ua.ScheduledAt == nil {
		return "Purchased"
	}

	now := time.Now()
	start := *ua.ScheduledAt

	// Determine session duration from the related Addon (0 if not loaded or not set)
	durationMinutes := 0
	if ua.Addon != nil && ua.Addon.Duration > 0 {
		durationMinutes = ua.Addon.Duration
	}

	end := start.Add(time.Duration(durationMinutes) * time.Minute)

	if now.Before(start) {
		return "Scheduled"
	}

	if durationMinutes > 0 && now.Before(end) {
		return "In Progress"
	}

	// If duration is 0 and we're past the start time, or if we're past end, it's completed
	if now.After(start) {
		return "Completed"
	}

	return "Scheduled"
}

// MarshalJSON overrides the default JSON marshalling so that the API always
// returns the dynamically computed status.
func (ua UserAddon) MarshalJSON() ([]byte, error) {
	type Alias UserAddon
	return json.Marshal(&struct {
		Alias
		Status string `json:"status"`
	}{
		Alias:  Alias(ua),
		Status: ua.CurrentStatus(),
	})
}

type Payment struct {
	ID                uint           `gorm:"primarykey" json:"id"`
	CreatedAt         time.Time      `json:"createdAt"`
	UpdatedAt         time.Time      `json:"updatedAt"`
	DeletedAt         gorm.DeletedAt `gorm:"index" json:"-"`
	UserID            uint           `json:"user_id" gorm:"index"`
	Amount            float64        `json:"amount"`
	Status            string         `json:"status"`                // Paid, Pending, Failed
	PaymentFor        string         `json:"payment_for"`           // Membership Plan, Add-On
	PlanID            *uint          `json:"plan_id" gorm:"index"`  // set when PaymentFor == "Membership Plan"
	AddonID           *uint          `json:"addon_id" gorm:"index"` // set when PaymentFor == "Add-On"
	RazorpayOrderID   string         `json:"razorpay_order_id"`
	RazorpayPaymentID string         `json:"razorpay_payment_id"`
	RazorpaySignature string         `json:"razorpay_signature"`
}

type Attendance struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	UserID    uint           `json:"user_id" gorm:"index"`
	User      *User          `json:"user" gorm:"foreignKey:UserID"`
	Date      time.Time      `json:"date" gorm:"type:date"`
	TimeIn    time.Time      `json:"time_in"`
	TimeOut   *time.Time     `json:"time_out"` // Nullable if they haven't checked out
	Source    string         `json:"source"`   // Manual, Biometric, QR
}

// GymQRToken holds the currently active QR token for a gym.
// The server rotates this token every 2 minutes.
// A member scans the QR code and submits the token to mark attendance.
type GymQRToken struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	GymID     uint           `json:"gym_id" gorm:"uniqueIndex"` // one active token per gym
	Token     string         `json:"token" gorm:"uniqueIndex"`
	ExpiresAt time.Time      `json:"expires_at"`
}

type WorkoutPlan struct {
	ID        uint              `gorm:"primarykey" json:"id"`
	CreatedAt time.Time         `json:"createdAt"`
	UpdatedAt time.Time         `json:"updatedAt"`
	DeletedAt gorm.DeletedAt    `gorm:"index" json:"-"`
	GymID     uint              `json:"gym_id" gorm:"index"`
	TrainerID uint              `json:"trainer_id" gorm:"index"`
	MemberID  uint              `json:"member_id" gorm:"index"`
	Title     string            `json:"title"`
	Exercises []WorkoutExercise `json:"exercises" gorm:"foreignKey:WorkoutPlanID"`
}

// WorkoutExercise is a single exercise row inside a WorkoutPlan.
type WorkoutExercise struct {
	ID            uint           `gorm:"primarykey" json:"id"`
	CreatedAt     time.Time      `json:"createdAt"`
	UpdatedAt     time.Time      `json:"updatedAt"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
	WorkoutPlanID uint           `json:"workout_plan_id" gorm:"index"`
	Name          string         `json:"name"`
}

func (g *Gym) BeforeCreate(tx *gorm.DB) (err error) {
	g.Name = strings.TrimSpace(g.Name)
	g.Address = strings.TrimSpace(g.Address)
	g.Whatsapp = strings.TrimSpace(g.Whatsapp)

	if g.Name == "" || g.Address == "" || g.Whatsapp == "" {
		return errors.New("name, address, and whatsapp are required")
	}

	g.Slug = slug.Make(g.Name)

	return nil
}
