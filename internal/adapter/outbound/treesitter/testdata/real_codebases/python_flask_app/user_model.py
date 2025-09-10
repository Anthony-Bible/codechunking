"""
User model and related functionality for the Flask application.
This module contains the User model, validation logic, and database operations.
"""

from datetime import datetime
from typing import Optional, List, Dict, Any
from dataclasses import dataclass, field
from email_validator import validate_email, EmailNotValidError
from werkzeug.security import generate_password_hash, check_password_hash
from sqlalchemy import Column, Integer, String, Boolean, DateTime, Text
from sqlalchemy.ext.declarative import declarative_base
from sqlalchemy.orm import relationship
import uuid
import re

Base = declarative_base()


class UserNotFoundError(Exception):
    """Raised when a user is not found in the database."""
    pass


class InvalidUserDataError(Exception):
    """Raised when user data is invalid."""
    pass


class DuplicateEmailError(Exception):
    """Raised when trying to create a user with an existing email."""
    pass


@dataclass
class UserProfile:
    """Represents a user's profile information."""
    first_name: str
    last_name: str
    bio: Optional[str] = None
    avatar_url: Optional[str] = None
    website_url: Optional[str] = None
    location: Optional[str] = None
    created_at: datetime = field(default_factory=datetime.utcnow)
    updated_at: datetime = field(default_factory=datetime.utcnow)

    def get_full_name(self) -> str:
        """Returns the user's full name."""
        return f"{self.first_name} {self.last_name}".strip()

    def update_profile(self, **kwargs) -> None:
        """Updates profile fields and sets updated_at timestamp."""
        for key, value in kwargs.items():
            if hasattr(self, key) and key not in ['created_at']:
                setattr(self, key, value)
        self.updated_at = datetime.utcnow()


class User(Base):
    """
    User model for the application.
    
    This class represents a user in the system with authentication,
    profile information, and various user-related operations.
    """
    
    __tablename__ = 'users'

    id = Column(Integer, primary_key=True)
    uuid = Column(String(36), unique=True, nullable=False, default=lambda: str(uuid.uuid4()))
    email = Column(String(255), unique=True, nullable=False, index=True)
    password_hash = Column(String(255), nullable=False)
    first_name = Column(String(100), nullable=False)
    last_name = Column(String(100), nullable=False)
    is_active = Column(Boolean, default=True, nullable=False)
    is_admin = Column(Boolean, default=False, nullable=False)
    email_verified = Column(Boolean, default=False, nullable=False)
    last_login_at = Column(DateTime, nullable=True)
    created_at = Column(DateTime, default=datetime.utcnow, nullable=False)
    updated_at = Column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow, nullable=False)
    deleted_at = Column(DateTime, nullable=True)
    
    # Profile information
    bio = Column(Text, nullable=True)
    avatar_url = Column(String(500), nullable=True)
    website_url = Column(String(500), nullable=True)
    location = Column(String(200), nullable=True)
    
    # Relationships would go here in a real application
    # posts = relationship("Post", back_populates="author")
    # comments = relationship("Comment", back_populates="user")

    def __init__(self, email: str, password: str, first_name: str, last_name: str, **kwargs):
        """Initialize a new User instance."""
        self.email = self.normalize_email(email)
        self.set_password(password)
        self.first_name = first_name
        self.last_name = last_name
        self.uuid = kwargs.get('uuid', str(uuid.uuid4()))
        
        # Set optional fields
        for field_name in ['is_active', 'is_admin', 'bio', 'avatar_url', 'website_url', 'location']:
            if field_name in kwargs:
                setattr(self, field_name, kwargs[field_name])

    def __repr__(self) -> str:
        """String representation of the User object."""
        return f"<User(id={self.id}, email='{self.email}', name='{self.get_full_name()}')>"

    def __str__(self) -> str:
        """Human-readable string representation."""
        return f"{self.get_full_name()} ({self.email})"

    @classmethod
    def normalize_email(cls, email: str) -> str:
        """Normalize email address to lowercase and strip whitespace."""
        if not email:
            raise InvalidUserDataError("Email address is required")
        return email.strip().lower()

    @classmethod
    def validate_email_address(cls, email: str) -> bool:
        """Validate email address format."""
        try:
            validate_email(email)
            return True
        except EmailNotValidError:
            return False

    @classmethod
    def validate_password_strength(cls, password: str) -> tuple[bool, List[str]]:
        """
        Validate password strength.
        
        Returns:
            tuple: (is_valid, list_of_errors)
        """
        errors = []
        
        if len(password) < 8:
            errors.append("Password must be at least 8 characters long")
        
        if not re.search(r'[A-Z]', password):
            errors.append("Password must contain at least one uppercase letter")
            
        if not re.search(r'[a-z]', password):
            errors.append("Password must contain at least one lowercase letter")
            
        if not re.search(r'\d', password):
            errors.append("Password must contain at least one digit")
            
        if not re.search(r'[!@#$%^&*()_+\-=\[\]{};\':"\\|,.<>?]', password):
            errors.append("Password must contain at least one special character")
        
        return len(errors) == 0, errors

    def set_password(self, password: str) -> None:
        """Set the user's password with proper hashing."""
        is_valid, errors = self.validate_password_strength(password)
        if not is_valid:
            raise InvalidUserDataError(f"Password validation failed: {', '.join(errors)}")
        
        self.password_hash = generate_password_hash(password, method='pbkdf2:sha256', salt_length=16)

    def check_password(self, password: str) -> bool:
        """Check if the provided password matches the user's password."""
        return check_password_hash(self.password_hash, password)

    def get_full_name(self) -> str:
        """Returns the user's full name."""
        return f"{self.first_name} {self.last_name}".strip()

    def get_display_name(self) -> str:
        """Returns the user's display name (full name or email if name is empty)."""
        full_name = self.get_full_name()
        return full_name if full_name else self.email

    def get_initials(self) -> str:
        """Returns the user's initials."""
        first_initial = self.first_name[0].upper() if self.first_name else ''
        last_initial = self.last_name[0].upper() if self.last_name else ''
        return f"{first_initial}{last_initial}"

    def is_deleted(self) -> bool:
        """Check if the user has been soft deleted."""
        return self.deleted_at is not None

    def soft_delete(self) -> None:
        """Soft delete the user by setting deleted_at timestamp."""
        self.deleted_at = datetime.utcnow()
        self.is_active = False

    def restore(self) -> None:
        """Restore a soft-deleted user."""
        self.deleted_at = None
        self.is_active = True

    def update_last_login(self) -> None:
        """Update the last login timestamp."""
        self.last_login_at = datetime.utcnow()

    def verify_email(self) -> None:
        """Mark the user's email as verified."""
        self.email_verified = True

    def get_profile(self) -> UserProfile:
        """Get the user's profile information."""
        return UserProfile(
            first_name=self.first_name,
            last_name=self.last_name,
            bio=self.bio,
            avatar_url=self.avatar_url,
            website_url=self.website_url,
            location=self.location,
            created_at=self.created_at,
            updated_at=self.updated_at
        )

    def update_profile(self, profile_data: Dict[str, Any]) -> None:
        """Update the user's profile information."""
        allowed_fields = ['first_name', 'last_name', 'bio', 'avatar_url', 'website_url', 'location']
        
        for field, value in profile_data.items():
            if field in allowed_fields:
                setattr(self, field, value)
        
        self.updated_at = datetime.utcnow()

    def validate_data(self) -> None:
        """Validate the user's data."""
        if not self.email:
            raise InvalidUserDataError("Email is required")
        
        if not self.validate_email_address(self.email):
            raise InvalidUserDataError("Invalid email address format")
        
        if not self.first_name or not self.first_name.strip():
            raise InvalidUserDataError("First name is required")
        
        if not self.last_name or not self.last_name.strip():
            raise InvalidUserDataError("Last name is required")
        
        if not self.password_hash:
            raise InvalidUserDataError("Password is required")

    def to_dict(self, include_sensitive: bool = False) -> Dict[str, Any]:
        """
        Convert user to dictionary representation.
        
        Args:
            include_sensitive: Whether to include sensitive information
        
        Returns:
            Dictionary representation of the user
        """
        user_dict = {
            'id': self.id,
            'uuid': self.uuid,
            'email': self.email,
            'first_name': self.first_name,
            'last_name': self.last_name,
            'full_name': self.get_full_name(),
            'display_name': self.get_display_name(),
            'initials': self.get_initials(),
            'is_active': self.is_active,
            'is_admin': self.is_admin,
            'email_verified': self.email_verified,
            'bio': self.bio,
            'avatar_url': self.avatar_url,
            'website_url': self.website_url,
            'location': self.location,
            'created_at': self.created_at.isoformat() if self.created_at else None,
            'updated_at': self.updated_at.isoformat() if self.updated_at else None,
            'is_deleted': self.is_deleted(),
        }
        
        if include_sensitive:
            user_dict.update({
                'password_hash': self.password_hash,
                'last_login_at': self.last_login_at.isoformat() if self.last_login_at else None,
                'deleted_at': self.deleted_at.isoformat() if self.deleted_at else None,
            })
        
        return user_dict

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> 'User':
        """Create a User instance from a dictionary."""
        required_fields = ['email', 'password', 'first_name', 'last_name']
        for field in required_fields:
            if field not in data:
                raise InvalidUserDataError(f"Missing required field: {field}")
        
        # Extract required fields
        user_data = {
            'email': data['email'],
            'password': data['password'],
            'first_name': data['first_name'],
            'last_name': data['last_name'],
        }
        
        # Add optional fields
        optional_fields = ['uuid', 'is_active', 'is_admin', 'bio', 'avatar_url', 'website_url', 'location']
        for field in optional_fields:
            if field in data:
                user_data[field] = data[field]
        
        return cls(**user_data)


class UserService:
    """Service class for user-related operations."""

    @staticmethod
    def create_user(email: str, password: str, first_name: str, last_name: str, **kwargs) -> User:
        """
        Create a new user with validation.
        
        Args:
            email: User's email address
            password: User's password
            first_name: User's first name
            last_name: User's last name
            **kwargs: Additional user fields
            
        Returns:
            New User instance
            
        Raises:
            InvalidUserDataError: If user data is invalid
        """
        # Normalize and validate email
        normalized_email = User.normalize_email(email)
        if not User.validate_email_address(normalized_email):
            raise InvalidUserDataError("Invalid email address")
        
        # Validate password
        is_valid, errors = User.validate_password_strength(password)
        if not is_valid:
            raise InvalidUserDataError(f"Password validation failed: {', '.join(errors)}")
        
        # Create user
        user = User(
            email=normalized_email,
            password=password,
            first_name=first_name.strip(),
            last_name=last_name.strip(),
            **kwargs
        )
        
        # Final validation
        user.validate_data()
        
        return user

    @staticmethod
    def authenticate_user(email: str, password: str, user_lookup_func) -> Optional[User]:
        """
        Authenticate a user by email and password.
        
        Args:
            email: User's email address
            password: User's password
            user_lookup_func: Function to lookup user by email
            
        Returns:
            User instance if authentication successful, None otherwise
        """
        try:
            normalized_email = User.normalize_email(email)
            user = user_lookup_func(normalized_email)
            
            if user and user.check_password(password) and user.is_active and not user.is_deleted():
                user.update_last_login()
                return user
            
        except Exception:
            # Log the exception in real implementation
            pass
        
        return None

    @staticmethod
    def update_user_profile(user: User, profile_data: Dict[str, Any]) -> None:
        """
        Update user profile with validation.
        
        Args:
            user: User instance to update
            profile_data: Dictionary of profile data to update
            
        Raises:
            InvalidUserDataError: If profile data is invalid
        """
        # Validate email if being updated
        if 'email' in profile_data:
            normalized_email = User.normalize_email(profile_data['email'])
            if not User.validate_email_address(normalized_email):
                raise InvalidUserDataError("Invalid email address")
            profile_data['email'] = normalized_email
        
        # Update user
        user.update_profile(profile_data)
        user.validate_data()