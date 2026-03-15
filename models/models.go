package models

import (
	"gorm.io/gorm"
	"time"
)

type User struct {
	gorm.Model
	Name                  string   `json:"name"`
	Email                 string   `json:"email" gorm:"unique"`
	Phone                 string   `json:"phone" gorm:"unique"`
	DOB                   string   `json:"dob"`
	Gender                string   `json:"gender"`
	PhotoURL              string   `json:"photo_url"`
	BiometricID           string   `json:"biometric_id"` // Simulated
	Role                  string   `json:"role"`         // SuperAdmin, GymAdmin, Trainer, Member
	GymID                 *uint    `json:"gym_id" gorm:"index"`
	TrainerID             *uint    `json:"trainer_id" gorm:"index"`
	Address               string   `json:"address"`
	EmergencyContactName  string   `json:"emergency_contact_name"`
	EmergencyContactPhone string   `json:"emergency_contact_phone"`
	BloodGroup            string   `json:"blood_group"`
	Height                *float64 `json:"height"`
	Weight                *float64 `json:"weight"`
	MedicalConditions     string   `json:"medical_conditions"`
}

type Gym struct {
	gorm.Model
	Name     string `json:"name"`
	Slug     string `json:"slug" gorm:"uniqueIndex"`
	Address  string `json:"address"`
	Whatsapp string `json:"whatsapp"`
	Users    []User `gorm:"foreignKey:GymID"`
}

type MembershipPlan struct {
	gorm.Model
	GymID          uint    `json:"gym_id" gorm:"index"`
	Name           string  `json:"name"`
	Price          float64 `json:"price"`
	DurationMonths int     `json:"duration_months"`
	IsActive       bool    `json:"is_active" gorm:"default:true"` // is its a active plan
}

type Subscription struct {
	gorm.Model
	UserID    uint      `json:"user_id" gorm:"index"`
	PlanID    uint      `json:"plan_id" gorm:"index"`
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
	Status    string    `json:"status"` // Active, Expired, Frozen
}

type Payment struct {
	gorm.Model
	UserID      uint      `json:"user_id" gorm:"index"`
	Amount      float64   `json:"amount"`
	PaymentDate time.Time `json:"payment_date"`
	Status      string    `json:"status"` // Paid, Pending, Failed
	PaymentFor  string    `json:"payment_for"` // Membership Plan, Add-On
}

type Attendance struct {
	gorm.Model
	UserID  uint       `json:"user_id" gorm:"index"`
	Date    time.Time  `json:"date" gorm:"type:date"`
	TimeIn  time.Time  `json:"time_in"`
	TimeOut *time.Time `json:"time_out"` // Nullable if they haven't checked out
	Source  string     `json:"source"`   // Manual, Biometric
}

type WorkoutPlan struct {
	gorm.Model
	GymID       uint   `json:"gym_id" gorm:"index"`
	TrainerID   uint   `json:"trainer_id" gorm:"index"`
	MemberID    uint   `json:"member_id" gorm:"index"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type Addon struct {
	gorm.Model
	GymID    uint    `json:"gym_id" gorm:"index"`
	Name     string  `json:"name"`
	Price    float64 `json:"price"`
	IsActive bool    `json:"is_active" gorm:"default:true"`
}
