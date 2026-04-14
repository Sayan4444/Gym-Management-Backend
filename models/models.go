package models

import (
	"errors"
	"strings"
	"time"

	"github.com/gosimple/slug"
	"gorm.io/gorm"
)

type User struct {
	ID                    uint           `gorm:"primarykey" json:"id"`
	CreatedAt             time.Time      `json:"createdAt"`
	UpdatedAt             time.Time      `json:"updatedAt"`
	DeletedAt             gorm.DeletedAt `gorm:"index" json:"-"`
	Name                  string         `json:"name"`
	Email                 string         `json:"email" gorm:"unique"`
	Phone                 string         `json:"phone" gorm:"unique"`
	DOB                   string         `json:"dob"`
	Gender                string         `json:"gender"`
	PhotoURL              string         `json:"photo_url"`
	BiometricID           string         `json:"biometric_id"` // Simulated
	Role                  string         `json:"role"`         // SuperAdmin, GymAdmin, Trainer, Member
	Address               string         `json:"address"`
	EmergencyContactName  string         `json:"emergency_contact_name"`
	EmergencyContactPhone string         `json:"emergency_contact_phone"`
	BloodGroup            string         `json:"blood_group"`
	Height                *float64       `json:"height"`
	Weight                *float64       `json:"weight"`
	MedicalConditions     string         `json:"medical_conditions"`
	GymID                 *uint          `json:"gym_id" gorm:"index"`
	Gym                   *Gym           `json:"gym" gorm:"foreignKey:GymID"`
	SubscriptionID        *uint          `json:"subscription_id" gorm:"index"`
	Subscription          *Subscription  `json:"subscription" gorm:"foreignKey:SubscriptionID"`
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
}

// deal provided by the gym and taken by the user
// Subcription taken by the user
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
	Status    string          `json:"status"` // Active, Expired, Paused, Upcoming
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
}

// UserAddon represents an addon purchased by a user
type UserAddon struct {
	ID          uint           `gorm:"primarykey" json:"id"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	UserID      uint           `json:"user_id" gorm:"index"`
	AddonID     uint           `json:"addon_id" gorm:"index"`
	PurchasedAt time.Time      `json:"purchased_at"`
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
// The server rotates this token every 30 seconds.
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
