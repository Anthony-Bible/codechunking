"""
Advanced Python class with comprehensive type hints, decorators, and documentation.
This module demonstrates modern Python features including async/await,
dataclasses, type annotations, and various decorators.
"""

from __future__ import annotations

import asyncio
import logging
import time
from abc import ABC, abstractmethod
from dataclasses import dataclass, field
from datetime import datetime, timedelta
from enum import Enum, auto
from functools import cached_property, lru_cache, wraps
from typing import (
    Any, AsyncGenerator, Awaitable, Callable, ClassVar, Dict, 
    Generic, Iterator, List, Optional, Protocol, Set, TypeVar, 
    Union, overload, runtime_checkable
)

import aiohttp
from pydantic import BaseModel, Field, validator


# Type variables for generic classes
T = TypeVar('T')
K = TypeVar('K')
V = TypeVar('V')

# Custom decorators with full type annotations

def performance_monitor(func: Callable[..., Any]) -> Callable[..., Any]:
    """
    Decorator to monitor function performance and log execution time.
    
    This decorator measures execution time and logs performance metrics
    for both synchronous and asynchronous functions.
    
    Args:
        func: The function to be monitored
        
    Returns:
        The wrapped function with performance monitoring
    """
    @wraps(func)
    async def async_wrapper(*args: Any, **kwargs: Any) -> Any:
        start_time = time.perf_counter()
        try:
            result = await func(*args, **kwargs)
            execution_time = time.perf_counter() - start_time
            logging.info(f"Async function {func.__name__} executed in {execution_time:.4f}s")
            return result
        except Exception as e:
            execution_time = time.perf_counter() - start_time
            logging.error(f"Async function {func.__name__} failed after {execution_time:.4f}s: {e}")
            raise
    
    @wraps(func)
    def sync_wrapper(*args: Any, **kwargs: Any) -> Any:
        start_time = time.perf_counter()
        try:
            result = func(*args, **kwargs)
            execution_time = time.perf_counter() - start_time
            logging.info(f"Function {func.__name__} executed in {execution_time:.4f}s")
            return result
        except Exception as e:
            execution_time = time.perf_counter() - start_time
            logging.error(f"Function {func.__name__} failed after {execution_time:.4f}s: {e}")
            raise
    
    if asyncio.iscoroutinefunction(func):
        return async_wrapper
    else:
        return sync_wrapper


def retry_on_failure(max_attempts: int = 3, delay: float = 1.0, backoff: float = 2.0) -> Callable:
    """
    Decorator to retry function calls on failure with exponential backoff.
    
    This decorator automatically retries failed function calls with
    configurable attempts and exponential backoff timing.
    
    Args:
        max_attempts: Maximum number of retry attempts
        delay: Initial delay between retries in seconds
        backoff: Backoff multiplier for delay between retries
        
    Returns:
        Decorator function
    """
    def decorator(func: Callable[..., T]) -> Callable[..., T]:
        @wraps(func)
        async def async_wrapper(*args: Any, **kwargs: Any) -> T:
            current_delay = delay
            last_exception = None
            
            for attempt in range(max_attempts):
                try:
                    return await func(*args, **kwargs)
                except Exception as e:
                    last_exception = e
                    if attempt < max_attempts - 1:
                        logging.warning(
                            f"Attempt {attempt + 1} failed for {func.__name__}: {e}. "
                            f"Retrying in {current_delay:.2f}s"
                        )
                        await asyncio.sleep(current_delay)
                        current_delay *= backoff
                    else:
                        logging.error(f"All {max_attempts} attempts failed for {func.__name__}")
            
            raise last_exception
        
        @wraps(func)
        def sync_wrapper(*args: Any, **kwargs: Any) -> T:
            current_delay = delay
            last_exception = None
            
            for attempt in range(max_attempts):
                try:
                    return func(*args, **kwargs)
                except Exception as e:
                    last_exception = e
                    if attempt < max_attempts - 1:
                        logging.warning(
                            f"Attempt {attempt + 1} failed for {func.__name__}: {e}. "
                            f"Retrying in {current_delay:.2f}s"
                        )
                        time.sleep(current_delay)
                        current_delay *= backoff
                    else:
                        logging.error(f"All {max_attempts} attempts failed for {func.__name__}")
            
            raise last_exception
        
        if asyncio.iscoroutinefunction(func):
            return async_wrapper
        else:
            return sync_wrapper
    
    return decorator


def validate_input(**validators: Dict[str, Callable[[Any], bool]]) -> Callable:
    """
    Decorator to validate function input parameters.
    
    Args:
        **validators: Dictionary mapping parameter names to validation functions
        
    Returns:
        Decorator function that validates inputs before execution
    """
    def decorator(func: Callable[..., T]) -> Callable[..., T]:
        @wraps(func)
        def wrapper(*args: Any, **kwargs: Any) -> T:
            # Get function signature for parameter mapping
            import inspect
            sig = inspect.signature(func)
            bound_args = sig.bind(*args, **kwargs)
            bound_args.apply_defaults()
            
            # Validate each parameter
            for param_name, validator_func in validators.items():
                if param_name in bound_args.arguments:
                    value = bound_args.arguments[param_name]
                    if not validator_func(value):
                        raise ValueError(f"Validation failed for parameter {param_name}: {value}")
            
            return func(*args, **kwargs)
        
        return wrapper
    return decorator


# Enums with comprehensive documentation

class UserStatus(Enum):
    """
    Enumeration of possible user account statuses.
    
    This enum defines all possible states that a user account
    can be in throughout its lifecycle.
    """
    PENDING = auto()      # User registered but not yet verified
    ACTIVE = auto()       # User is active and can use the system
    INACTIVE = auto()     # User is temporarily deactivated
    SUSPENDED = auto()    # User is suspended due to violations
    DELETED = auto()      # User account has been deleted


class UserRole(Enum):
    """User role enumeration with associated permissions."""
    GUEST = "guest"           # Limited read-only access
    USER = "user"             # Standard user privileges
    MODERATOR = "moderator"   # Can moderate content
    ADMIN = "admin"           # Full administrative access
    SUPER_ADMIN = "super"     # System-level administration


# Protocol definitions for type checking

@runtime_checkable
class Cacheable(Protocol):
    """Protocol for objects that can be cached."""
    
    def cache_key(self) -> str:
        """Return a unique cache key for this object."""
        ...
    
    def expires_at(self) -> Optional[datetime]:
        """Return when this cached object expires."""
        ...


@runtime_checkable
class Serializable(Protocol):
    """Protocol for objects that can be serialized."""
    
    def to_dict(self) -> Dict[str, Any]:
        """Convert object to dictionary representation."""
        ...
    
    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> Serializable:
        """Create object from dictionary representation."""
        ...


# Dataclasses with comprehensive type annotations

@dataclass(frozen=True)
class UserPreferences:
    """
    Immutable user preferences data class.
    
    This dataclass stores user-specific settings and preferences
    using modern Python dataclass features with type annotations.
    """
    
    # Theme and UI preferences
    theme: str = field(default="light", metadata={"choices": ["light", "dark", "auto"]})
    language: str = field(default="en", metadata={"pattern": r"[a-z]{2}"})
    timezone: str = field(default="UTC")
    
    # Notification preferences
    email_notifications: bool = True
    push_notifications: bool = True
    sms_notifications: bool = False
    
    # Privacy settings
    profile_visible: bool = True
    show_email: bool = False
    show_last_seen: bool = True
    
    # Feature flags
    beta_features: Set[str] = field(default_factory=set)
    
    # Custom settings as JSON-serializable dict
    custom_settings: Dict[str, Any] = field(default_factory=dict)
    
    def __post_init__(self) -> None:
        """Validate preferences after initialization."""
        if self.theme not in ["light", "dark", "auto"]:
            raise ValueError(f"Invalid theme: {self.theme}")
        
        if len(self.language) != 2:
            raise ValueError(f"Language must be 2 characters: {self.language}")


@dataclass
class UserActivity:
    """
    User activity tracking data class.
    
    This mutable dataclass tracks user activity metrics
    and provides methods for activity analysis.
    """
    
    user_id: str
    last_login: Optional[datetime] = None
    login_count: int = 0
    page_views: int = 0
    actions_performed: int = 0
    session_duration: timedelta = field(default_factory=lambda: timedelta(0))
    
    # Activity tracking
    activity_log: List[Dict[str, Any]] = field(default_factory=list)
    
    def record_activity(self, action: str, metadata: Optional[Dict[str, Any]] = None) -> None:
        """Record a user activity with timestamp and metadata."""
        activity_entry = {
            "action": action,
            "timestamp": datetime.utcnow().isoformat(),
            "metadata": metadata or {}
        }
        self.activity_log.append(activity_entry)
        self.actions_performed += 1
    
    @cached_property
    def average_session_duration(self) -> timedelta:
        """Calculate average session duration."""
        if self.login_count == 0:
            return timedelta(0)
        return self.session_duration / self.login_count


# Base classes with comprehensive type annotations

class BaseEntity(ABC, Generic[T]):
    """
    Abstract base class for all domain entities.
    
    This generic base class provides common functionality
    for all domain entities including validation, serialization,
    and audit trail features.
    """
    
    # Class-level configuration
    _registry: ClassVar[Dict[str, type]] = {}
    _validation_rules: ClassVar[Dict[str, List[Callable]]] = {}
    
    def __init__(self, entity_id: T, created_at: Optional[datetime] = None) -> None:
        """
        Initialize base entity with ID and creation timestamp.
        
        Args:
            entity_id: Unique identifier for the entity
            created_at: Entity creation timestamp (defaults to now)
        """
        self.id: T = entity_id
        self.created_at: datetime = created_at or datetime.utcnow()
        self.updated_at: datetime = self.created_at
        self.version: int = 1
    
    @abstractmethod
    def validate(self) -> bool:
        """
        Validate the entity's current state.
        
        Returns:
            True if entity is valid, raises exception otherwise
        """
        pass
    
    @abstractmethod
    def to_dict(self) -> Dict[str, Any]:
        """Convert entity to dictionary representation."""
        pass
    
    def touch(self) -> None:
        """Update the entity's modification timestamp."""
        self.updated_at = datetime.utcnow()
        self.version += 1
    
    def __eq__(self, other: object) -> bool:
        """Check equality based on entity ID."""
        if not isinstance(other, BaseEntity):
            return NotImplemented
        return self.id == other.id
    
    def __hash__(self) -> int:
        """Hash based on entity ID."""
        return hash(self.id)


class AsyncResourceManager(Generic[K, V]):
    """
    Generic async resource manager with comprehensive type safety.
    
    This class demonstrates advanced generic typing with async context
    management and resource lifecycle handling.
    """
    
    def __init__(self, resource_factory: Callable[[], Awaitable[V]]) -> None:
        """
        Initialize resource manager with factory function.
        
        Args:
            resource_factory: Async function that creates resources
        """
        self._factory: Callable[[], Awaitable[V]] = resource_factory
        self._resources: Dict[K, V] = {}
        self._locks: Dict[K, asyncio.Lock] = {}
        self._cleanup_tasks: Set[asyncio.Task] = set()
    
    async def acquire(self, key: K) -> V:
        """
        Acquire a resource by key, creating if necessary.
        
        Args:
            key: Resource identifier
            
        Returns:
            The requested resource
        """
        if key not in self._locks:
            self._locks[key] = asyncio.Lock()
        
        async with self._locks[key]:
            if key not in self._resources:
                self._resources[key] = await self._factory()
            return self._resources[key]
    
    async def release(self, key: K) -> None:
        """
        Release a resource and clean up.
        
        Args:
            key: Resource identifier to release
        """
        if key in self._resources:
            resource = self._resources.pop(key)
            
            # Schedule cleanup if resource supports it
            if hasattr(resource, 'close'):
                task = asyncio.create_task(resource.close())
                self._cleanup_tasks.add(task)
                task.add_done_callback(self._cleanup_tasks.discard)
    
    async def cleanup_all(self) -> None:
        """Clean up all resources and pending tasks."""
        # Release all resources
        for key in list(self._resources.keys()):
            await self.release(key)
        
        # Wait for cleanup tasks to complete
        if self._cleanup_tasks:
            await asyncio.gather(*self._cleanup_tasks, return_exceptions=True)


# Main class with comprehensive decorators and type annotations

class AdvancedUserManager(BaseEntity[str], Cacheable, Serializable):
    """
    Advanced user management class demonstrating modern Python features.
    
    This class showcases:
    - Comprehensive type annotations
    - Multiple inheritance and mixins
    - Async/await patterns
    - Decorators for cross-cutting concerns
    - Property decorators and caching
    - Context managers
    - Generator and async generator methods
    """
    
    # Class variables with type annotations
    _instance_counter: ClassVar[int] = 0
    _global_config: ClassVar[Dict[str, Any]] = {
        "max_concurrent_operations": 10,
        "default_timeout": 30.0,
        "cache_ttl": 3600,
    }
    
    def __init__(
        self,
        manager_id: str,
        session: Optional[aiohttp.ClientSession] = None,
        preferences: Optional[UserPreferences] = None,
    ) -> None:
        """
        Initialize the advanced user manager.
        
        Args:
            manager_id: Unique identifier for this manager instance
            session: Optional HTTP client session for API calls
            preferences: Optional user preferences configuration
        """
        super().__init__(manager_id)
        
        # Instance variables with type annotations
        self._session: Optional[aiohttp.ClientSession] = session
        self._preferences: UserPreferences = preferences or UserPreferences()
        self._users: Dict[str, Dict[str, Any]] = {}
        self._activity_tracker: Dict[str, UserActivity] = {}
        self._operation_semaphore: asyncio.Semaphore = asyncio.Semaphore(
            self._global_config["max_concurrent_operations"]
        )
        
        # Increment instance counter
        AdvancedUserManager._instance_counter += 1
        self._instance_number: int = self._instance_counter
        
        # Initialize async components
        self._initialized: bool = False
        self._background_tasks: Set[asyncio.Task] = set()
    
    @property
    def is_initialized(self) -> bool:
        """Check if the manager is fully initialized."""
        return self._initialized
    
    @cached_property
    def instance_info(self) -> Dict[str, Any]:
        """Get cached instance information."""
        return {
            "id": self.id,
            "instance_number": self._instance_number,
            "created_at": self.created_at.isoformat(),
            "preferences": self._preferences.to_dict() if hasattr(self._preferences, 'to_dict') else None,
        }
    
    @performance_monitor
    async def initialize(self) -> None:
        """
        Initialize the user manager asynchronously.
        
        This method sets up the HTTP session and performs
        any necessary initialization tasks.
        """
        if self._initialized:
            return
        
        if not self._session:
            self._session = aiohttp.ClientSession(
                timeout=aiohttp.ClientTimeout(
                    total=self._global_config["default_timeout"]
                )
            )
        
        # Start background maintenance tasks
        maintenance_task = asyncio.create_task(self._background_maintenance())
        self._background_tasks.add(maintenance_task)
        maintenance_task.add_done_callback(self._background_tasks.discard)
        
        self._initialized = True
        logging.info(f"UserManager {self.id} initialized successfully")
    
    @retry_on_failure(max_attempts=3, delay=1.0, backoff=2.0)
    @performance_monitor
    async def create_user(
        self,
        user_data: Dict[str, Any],
        *,
        validate: bool = True,
        notify: bool = True,
    ) -> Dict[str, Any]:
        """
        Create a new user with comprehensive validation and error handling.
        
        Args:
            user_data: Dictionary containing user information
            validate: Whether to perform validation before creation
            notify: Whether to send notification after creation
            
        Returns:
            Created user data with generated fields
            
        Raises:
            ValueError: If validation fails
            RuntimeError: If creation fails due to system error
        """
        async with self._operation_semaphore:
            if not self._initialized:
                await self.initialize()
            
            # Validate input if requested
            if validate:
                await self._validate_user_data(user_data)
            
            # Generate user ID and timestamps
            user_id = f"user_{len(self._users):06d}_{int(time.time())}"
            now = datetime.utcnow()
            
            # Create complete user record
            complete_user_data = {
                **user_data,
                "id": user_id,
                "status": UserStatus.PENDING.name,
                "role": UserRole.USER.value,
                "created_at": now.isoformat(),
                "updated_at": now.isoformat(),
                "version": 1,
            }
            
            # Store user
            self._users[user_id] = complete_user_data
            
            # Initialize activity tracking
            self._activity_tracker[user_id] = UserActivity(user_id=user_id)
            
            # Send notification if requested
            if notify:
                await self._send_user_notification(
                    user_id, 
                    "welcome", 
                    {"user_data": complete_user_data}
                )
            
            logging.info(f"Created user {user_id} successfully")
            return complete_user_data
    
    @performance_monitor
    @lru_cache(maxsize=128)
    def get_user_sync(self, user_id: str) -> Optional[Dict[str, Any]]:
        """
        Synchronously get user data (cached).
        
        Args:
            user_id: User identifier
            
        Returns:
            User data if found, None otherwise
        """
        return self._users.get(user_id)
    
    async def get_user_async(self, user_id: str) -> Optional[Dict[str, Any]]:
        """
        Asynchronously get user data with activity tracking.
        
        Args:
            user_id: User identifier
            
        Returns:
            User data if found, None otherwise
        """
        user_data = self.get_user_sync(user_id)
        
        if user_data and user_id in self._activity_tracker:
            self._activity_tracker[user_id].record_activity(
                "profile_viewed",
                {"accessed_by": self.id, "timestamp": datetime.utcnow().isoformat()}
            )
        
        return user_data
    
    @overload
    async def update_user(self, user_id: str, updates: Dict[str, Any]) -> bool:
        ...
    
    @overload
    async def update_user(self, user_id: str, **kwargs: Any) -> bool:
        ...
    
    async def update_user(
        self, 
        user_id: str, 
        updates: Optional[Dict[str, Any]] = None, 
        **kwargs: Any
    ) -> bool:
        """
        Update user data with type-safe overloads.
        
        Args:
            user_id: User identifier
            updates: Dictionary of updates (alternative to kwargs)
            **kwargs: Individual field updates
            
        Returns:
            True if update was successful, False otherwise
        """
        if user_id not in self._users:
            return False
        
        # Merge updates from dict and kwargs
        all_updates = {**(updates or {}), **kwargs}
        
        if not all_updates:
            return True
        
        # Apply updates
        user_data = self._users[user_id]
        for key, value in all_updates.items():
            if key not in ["id", "created_at"]:  # Protect immutable fields
                user_data[key] = value
        
        # Update metadata
        user_data["updated_at"] = datetime.utcnow().isoformat()
        user_data["version"] = user_data.get("version", 1) + 1
        
        # Record activity
        if user_id in self._activity_tracker:
            self._activity_tracker[user_id].record_activity(
                "profile_updated",
                {"updated_fields": list(all_updates.keys())}
            )
        
        self.touch()
        return True
    
    async def list_users(
        self,
        *,
        status_filter: Optional[UserStatus] = None,
        role_filter: Optional[UserRole] = None,
        limit: int = 100,
        offset: int = 0,
    ) -> AsyncGenerator[Dict[str, Any], None]:
        """
        Asynchronously generate filtered user list.
        
        Args:
            status_filter: Optional status to filter by
            role_filter: Optional role to filter by
            limit: Maximum number of users to return
            offset: Number of users to skip
            
        Yields:
            User data dictionaries matching the filters
        """
        count = 0
        skipped = 0
        
        for user_data in self._users.values():
            # Apply filters
            if status_filter and user_data.get("status") != status_filter.name:
                continue
            
            if role_filter and user_data.get("role") != role_filter.value:
                continue
            
            # Apply offset
            if skipped < offset:
                skipped += 1
                continue
            
            # Apply limit
            if count >= limit:
                break
            
            yield user_data
            count += 1
            
            # Yield control periodically for better async behavior
            if count % 10 == 0:
                await asyncio.sleep(0)
    
    def user_statistics(self) -> Iterator[Tuple[str, Dict[str, Any]]]:
        """
        Generate user statistics synchronously.
        
        Yields:
            Tuples of (user_id, statistics_dict)
        """
        for user_id, activity in self._activity_tracker.items():
            if user_id in self._users:
                user_data = self._users[user_id]
                stats = {
                    "login_count": activity.login_count,
                    "actions_performed": activity.actions_performed,
                    "average_session_duration": activity.average_session_duration.total_seconds(),
                    "account_age_days": (
                        datetime.utcnow() - datetime.fromisoformat(user_data["created_at"])
                    ).days,
                    "status": user_data.get("status"),
                    "role": user_data.get("role"),
                }
                yield user_id, stats
    
    async def bulk_operation(
        self,
        operation: str,
        user_ids: List[str],
        **operation_params: Any,
    ) -> Dict[str, Union[bool, str]]:
        """
        Perform bulk operations on multiple users concurrently.
        
        Args:
            operation: Operation name ("update", "delete", "activate", etc.)
            user_ids: List of user IDs to operate on
            **operation_params: Parameters for the operation
            
        Returns:
            Dictionary mapping user_ids to operation results
        """
        async def perform_single_operation(user_id: str) -> Union[bool, str]:
            try:
                if operation == "update":
                    return await self.update_user(user_id, operation_params)
                elif operation == "delete":
                    return await self._delete_user(user_id)
                elif operation == "activate":
                    return await self.update_user(user_id, {"status": UserStatus.ACTIVE.name})
                else:
                    return f"Unknown operation: {operation}"
            except Exception as e:
                return f"Error: {str(e)}"
        
        # Execute operations concurrently with semaphore
        tasks = [
            asyncio.create_task(perform_single_operation(user_id))
            for user_id in user_ids
        ]
        
        results = await asyncio.gather(*tasks, return_exceptions=True)
        
        return {
            user_id: result if not isinstance(result, Exception) else f"Exception: {result}"
            for user_id, result in zip(user_ids, results)
        }
    
    # Context manager support
    async def __aenter__(self) -> AdvancedUserManager:
        """Async context manager entry."""
        await self.initialize()
        return self
    
    async def __aexit__(self, exc_type, exc_val, exc_tb) -> None:
        """Async context manager exit with cleanup."""
        await self.cleanup()
    
    async def cleanup(self) -> None:
        """Clean up resources and background tasks."""
        # Cancel background tasks
        for task in self._background_tasks:
            task.cancel()
        
        if self._background_tasks:
            await asyncio.gather(*self._background_tasks, return_exceptions=True)
        
        # Close HTTP session
        if self._session:
            await self._session.close()
            self._session = None
        
        self._initialized = False
        logging.info(f"UserManager {self.id} cleaned up successfully")
    
    # Implementation of abstract and protocol methods
    
    def validate(self) -> bool:
        """Validate the user manager state."""
        if not self.id:
            raise ValueError("Manager ID is required")
        
        if not isinstance(self._preferences, UserPreferences):
            raise ValueError("Invalid preferences type")
        
        return True
    
    def to_dict(self) -> Dict[str, Any]:
        """Convert manager to dictionary representation."""
        return {
            "id": self.id,
            "created_at": self.created_at.isoformat(),
            "updated_at": self.updated_at.isoformat(),
            "version": self.version,
            "instance_number": self._instance_number,
            "user_count": len(self._users),
            "is_initialized": self._initialized,
            "preferences": self._preferences.__dict__,
        }
    
    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> AdvancedUserManager:
        """Create manager from dictionary representation."""
        preferences = UserPreferences(**data.get("preferences", {}))
        manager = cls(
            manager_id=data["id"],
            preferences=preferences,
        )
        manager.version = data.get("version", 1)
        return manager
    
    def cache_key(self) -> str:
        """Return cache key for this manager."""
        return f"user_manager:{self.id}:{self.version}"
    
    def expires_at(self) -> Optional[datetime]:
        """Return cache expiration time."""
        return self.updated_at + timedelta(seconds=self._global_config["cache_ttl"])
    
    # Private helper methods
    
    @validate_input(
        user_data=lambda data: isinstance(data, dict) and "email" in data,
    )
    async def _validate_user_data(self, user_data: Dict[str, Any]) -> None:
        """Validate user data before creation."""
        required_fields = ["email", "first_name", "last_name"]
        
        for field in required_fields:
            if not user_data.get(field):
                raise ValueError(f"Required field missing: {field}")
        
        # Validate email format (simplified)
        email = user_data["email"]
        if "@" not in email or "." not in email:
            raise ValueError(f"Invalid email format: {email}")
    
    async def _send_user_notification(
        self,
        user_id: str,
        notification_type: str,
        context: Dict[str, Any],
    ) -> None:
        """Send notification to user (simulated)."""
        # Simulate async notification sending
        await asyncio.sleep(0.1)
        
        notification_data = {
            "user_id": user_id,
            "type": notification_type,
            "context": context,
            "sent_at": datetime.utcnow().isoformat(),
        }
        
        logging.info(f"Sent {notification_type} notification to {user_id}")
    
    async def _delete_user(self, user_id: str) -> bool:
        """Delete user from the system."""
        if user_id in self._users:
            del self._users[user_id]
        
        if user_id in self._activity_tracker:
            del self._activity_tracker[user_id]
        
        return True
    
    async def _background_maintenance(self) -> None:
        """Run background maintenance tasks."""
        while True:
            try:
                # Perform periodic maintenance
                await asyncio.sleep(60)  # Run every minute
                
                # Clean up expired data, update statistics, etc.
                expired_users = [
                    user_id for user_id, user_data in self._users.items()
                    if self._is_user_expired(user_data)
                ]
                
                for user_id in expired_users:
                    await self._delete_user(user_id)
                
                if expired_users:
                    logging.info(f"Cleaned up {len(expired_users)} expired users")
                
            except asyncio.CancelledError:
                break
            except Exception as e:
                logging.error(f"Background maintenance error: {e}")
    
    def _is_user_expired(self, user_data: Dict[str, Any]) -> bool:
        """Check if user data is expired (simplified logic)."""
        # Simplified expiration logic
        created_at = datetime.fromisoformat(user_data["created_at"])
        return (datetime.utcnow() - created_at).days > 365  # 1 year expiration
    
    # String representation methods
    def __repr__(self) -> str:
        return f"AdvancedUserManager(id={self.id!r}, users={len(self._users)})"
    
    def __str__(self) -> str:
        return f"User Manager {self.id} with {len(self._users)} users"


# Factory function with comprehensive type annotations
async def create_user_manager(
    manager_id: str,
    *,
    preferences: Optional[UserPreferences] = None,
    session_config: Optional[Dict[str, Any]] = None,
) -> AdvancedUserManager:
    """
    Factory function to create and initialize a user manager.
    
    Args:
        manager_id: Unique identifier for the manager
        preferences: Optional user preferences
        session_config: Optional HTTP session configuration
        
    Returns:
        Initialized AdvancedUserManager instance
    """
    # Create HTTP session with custom config
    session = None
    if session_config:
        timeout = aiohttp.ClientTimeout(
            total=session_config.get("timeout", 30.0)
        )
        session = aiohttp.ClientSession(timeout=timeout)
    
    # Create and initialize manager
    manager = AdvancedUserManager(
        manager_id=manager_id,
        session=session,
        preferences=preferences,
    )
    
    await manager.initialize()
    return manager


# Main execution example with proper async handling
if __name__ == "__main__":
    async def main() -> None:
        """Main execution function demonstrating the user manager."""
        # Set up logging
        logging.basicConfig(
            level=logging.INFO,
            format="%(asctime)s - %(name)s - %(levelname)s - %(message)s"
        )
        
        # Create preferences
        prefs = UserPreferences(
            theme="dark",
            language="en",
            email_notifications=True,
            beta_features={"advanced_search", "real_time_sync"}
        )
        
        # Create user manager using context manager
        async with await create_user_manager("main_manager", preferences=prefs) as manager:
            # Create some test users
            users_to_create = [
                {
                    "email": f"user{i}@example.com",
                    "first_name": f"User{i}",
                    "last_name": "Test",
                    "role": UserRole.USER.value if i % 3 != 0 else UserRole.ADMIN.value,
                }
                for i in range(1, 6)
            ]
            
            # Create users concurrently
            create_tasks = [
                manager.create_user(user_data)
                for user_data in users_to_create
            ]
            
            created_users = await asyncio.gather(*create_tasks)
            print(f"Created {len(created_users)} users")
            
            # Demonstrate async iteration
            print("\nListing all users:")
            async for user in manager.list_users(limit=10):
                print(f"  - {user['first_name']} {user['last_name']} ({user['email']})")
            
            # Generate statistics
            print("\nUser Statistics:")
            for user_id, stats in manager.user_statistics():
                print(f"  {user_id}: {stats}")
            
            # Demonstrate bulk operations
            user_ids = [user["id"] for user in created_users[:3]]
            results = await manager.bulk_operation(
                "activate",
                user_ids,
            )
            print(f"\nBulk activation results: {results}")
    
    # Run the async main function
    asyncio.run(main())