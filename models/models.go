package models

import (
	"time"
	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	Name         string `json:"name"`
	Email        string `json:"email" gorm:"unique"`
	PasswordHash string `json:"-"`
	Role         string `json:"role"` // SuperAdmin, GymAdmin, Trainer, Member
	GymID        *uint  `json:"gym_id"` // Nullable for SuperAdmin
}

type Gym struct {
	gorm.Model
	Name    string `json:"name"`
	Address string `json:"address"`
	Users   []User `gorm:"foreignKey:GymID"`
}

type MemberProfile struct {
	gorm.Model
	UserID      uint   `json:"user_id" gorm:"unique"`
	User        User   `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Phone       string `json:"phone"`
	DOB         string `json:"dob"`
	Gender      string `json:"gender"`
	BiometricID string `json:"biometric_id"` // Simulated
	JoinDate    time.Time `json:"join_date"`
}

type MembershipPlan struct {
	gorm.Model
	GymID          uint    `json:"gym_id"`
	Name           string  `json:"name"`
	DurationMonths int     `json:"duration_months"`
	Price          float64 `json:"price"`
	IsActive       bool    `json:"is_active" gorm:"default:true"`
}

type Subscription struct {
	gorm.Model
	UserID         uint           `json:"user_id"`
	User           User           `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	PlanID         uint           `json:"plan_id"`
	Plan           MembershipPlan `gorm:"constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;"`
	StartDate      time.Time      `json:"start_date"`
	EndDate        time.Time      `json:"end_date"`
	Status         string         `json:"status"` // Active, Expired, Frozen
}

type Payment struct {
	gorm.Model
	SubscriptionID uint         `json:"subscription_id"`
	Subscription   Subscription `gorm:"constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;"`
	Amount         float64      `json:"amount"`
	PaymentDate    time.Time    `json:"payment_date"`
	Status         string       `json:"status"` // Paid, Pending, Failed
}

type Attendance struct {
	gorm.Model
	UserID   uint      `json:"user_id"`
	User     User      `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Date     time.Time `json:"date" gorm:"type:date"`
	TimeIn   time.Time `json:"time_in"`
	TimeOut  *time.Time `json:"time_out"` // Nullable if they haven't checked out
	Source   string    `json:"source"`   // Manual, Biometric
}
