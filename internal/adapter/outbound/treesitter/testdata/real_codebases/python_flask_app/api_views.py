"""
Flask API views for user management.
This module contains all the REST API endpoints for user operations.
"""

from flask import Blueprint, request, jsonify, current_app, g
from flask_jwt_extended import jwt_required, get_jwt_identity, create_access_token, create_refresh_token
from marshmallow import Schema, fields, validate, ValidationError, post_load
from functools import wraps
from typing import Dict, Any, Optional, List, Tuple
import logging
from datetime import datetime, timedelta

from .user_model import User, UserService, UserNotFoundError, InvalidUserDataError, DuplicateEmailError

# Set up logging
logger = logging.getLogger(__name__)

# Create Blueprint
api_bp = Blueprint('api', __name__, url_prefix='/api/v1')


class APIError(Exception):
    """Base API error class."""
    status_code = 500
    message = "Internal server error"

    def __init__(self, message: str = None, status_code: int = None, payload: Dict = None):
        super().__init__()
        if message is not None:
            self.message = message
        if status_code is not None:
            self.status_code = status_code
        self.payload = payload or {}

    def to_dict(self) -> Dict[str, Any]:
        """Convert error to dictionary."""
        result = {'error': {'message': self.message, 'code': self.status_code}}
        if self.payload:
            result['error']['details'] = self.payload
        return result


class ValidationAPIError(APIError):
    """API error for validation failures."""
    status_code = 400


class NotFoundAPIError(APIError):
    """API error for not found resources."""
    status_code = 404


class ConflictAPIError(APIError):
    """API error for conflicts."""
    status_code = 409


class UnauthorizedAPIError(APIError):
    """API error for unauthorized access."""
    status_code = 401


# Marshmallow Schemas for request/response validation

class UserRegistrationSchema(Schema):
    """Schema for user registration."""
    email = fields.Email(required=True, validate=validate.Length(max=255))
    password = fields.Str(required=True, validate=validate.Length(min=8, max=128))
    first_name = fields.Str(required=True, validate=validate.Length(min=1, max=100))
    last_name = fields.Str(required=True, validate=validate.Length(min=1, max=100))
    bio = fields.Str(missing=None, validate=validate.Length(max=1000))
    website_url = fields.Url(missing=None, validate=validate.Length(max=500))
    location = fields.Str(missing=None, validate=validate.Length(max=200))

    @post_load
    def make_user_data(self, data, **kwargs):
        """Process validated data."""
        return data


class UserLoginSchema(Schema):
    """Schema for user login."""
    email = fields.Email(required=True)
    password = fields.Str(required=True)
    remember_me = fields.Bool(missing=False)


class UserUpdateSchema(Schema):
    """Schema for user profile updates."""
    first_name = fields.Str(validate=validate.Length(min=1, max=100))
    last_name = fields.Str(validate=validate.Length(min=1, max=100))
    bio = fields.Str(validate=validate.Length(max=1000), allow_none=True)
    website_url = fields.Url(validate=validate.Length(max=500), allow_none=True)
    location = fields.Str(validate=validate.Length(max=200), allow_none=True)


class UserResponseSchema(Schema):
    """Schema for user response data."""
    id = fields.Int()
    uuid = fields.Str()
    email = fields.Email()
    first_name = fields.Str()
    last_name = fields.Str()
    full_name = fields.Str()
    display_name = fields.Str()
    is_active = fields.Bool()
    is_admin = fields.Bool()
    email_verified = fields.Bool()
    bio = fields.Str(allow_none=True)
    website_url = fields.Str(allow_none=True)
    location = fields.Str(allow_none=True)
    created_at = fields.DateTime()
    updated_at = fields.DateTime()


# Decorators

def handle_api_errors(f):
    """Decorator to handle API errors consistently."""
    @wraps(f)
    def decorated_function(*args, **kwargs):
        try:
            return f(*args, **kwargs)
        except APIError as e:
            return jsonify(e.to_dict()), e.status_code
        except ValidationError as e:
            error = ValidationAPIError("Validation failed", payload=e.messages)
            return jsonify(error.to_dict()), error.status_code
        except UserNotFoundError:
            error = NotFoundAPIError("User not found")
            return jsonify(error.to_dict()), error.status_code
        except InvalidUserDataError as e:
            error = ValidationAPIError(str(e))
            return jsonify(error.to_dict()), error.status_code
        except DuplicateEmailError:
            error = ConflictAPIError("Email address already exists")
            return jsonify(error.to_dict()), error.status_code
        except Exception as e:
            logger.exception("Unexpected error in API endpoint")
            error = APIError("Internal server error")
            return jsonify(error.to_dict()), error.status_code
    
    return decorated_function


def validate_json_request(schema_class):
    """Decorator to validate JSON request data."""
    def decorator(f):
        @wraps(f)
        def decorated_function(*args, **kwargs):
            if not request.is_json:
                raise ValidationAPIError("Content-Type must be application/json")
            
            schema = schema_class()
            try:
                validated_data = schema.load(request.json or {})
                g.validated_data = validated_data
                return f(*args, **kwargs)
            except ValidationError as e:
                raise ValidationAPIError("Request validation failed", payload=e.messages)
        
        return decorated_function
    return decorator


def admin_required(f):
    """Decorator to require admin privileges."""
    @wraps(f)
    @jwt_required()
    def decorated_function(*args, **kwargs):
        current_user_id = get_jwt_identity()
        # In a real app, you'd look up the user and check admin status
        # user = User.query.get(current_user_id)
        # if not user or not user.is_admin:
        #     raise UnauthorizedAPIError("Admin privileges required")
        return f(*args, **kwargs)
    
    return decorated_function


# API Endpoints

@api_bp.route('/auth/register', methods=['POST'])
@handle_api_errors
@validate_json_request(UserRegistrationSchema)
def register_user():
    """
    Register a new user.
    
    ---
    post:
      tags: [Authentication]
      summary: Register a new user
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/UserRegistrationSchema'
      responses:
        201:
          description: User created successfully
        400:
          description: Validation error
        409:
          description: Email already exists
    """
    user_data = g.validated_data
    
    # Check if user already exists (simulate database check)
    # In real implementation: existing_user = User.query.filter_by(email=user_data['email']).first()
    # if existing_user:
    #     raise DuplicateEmailError()
    
    # Create new user
    user = UserService.create_user(**user_data)
    
    # In real implementation: db.session.add(user), db.session.commit()
    
    # Create access tokens
    access_token = create_access_token(identity=user.id)
    refresh_token = create_refresh_token(identity=user.id)
    
    # Serialize user data
    user_schema = UserResponseSchema()
    user_response = user_schema.dump(user.to_dict())
    
    response = {
        'message': 'User registered successfully',
        'user': user_response,
        'tokens': {
            'access_token': access_token,
            'refresh_token': refresh_token
        }
    }
    
    logger.info(f"New user registered: {user.email}")
    return jsonify(response), 201


@api_bp.route('/auth/login', methods=['POST'])
@handle_api_errors
@validate_json_request(UserLoginSchema)
def login_user():
    """
    Authenticate user and return JWT tokens.
    
    ---
    post:
      tags: [Authentication]
      summary: User login
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/UserLoginSchema'
      responses:
        200:
          description: Login successful
        401:
          description: Invalid credentials
    """
    login_data = g.validated_data
    
    # Simulate user lookup and authentication
    def mock_user_lookup(email: str) -> Optional[User]:
        # In real implementation: return User.query.filter_by(email=email).first()
        # For this example, create a mock user
        if email == "test@example.com":
            return User(
                email=email,
                password="TestPassword123!",
                first_name="Test",
                last_name="User"
            )
        return None
    
    user = UserService.authenticate_user(
        email=login_data['email'],
        password=login_data['password'],
        user_lookup_func=mock_user_lookup
    )
    
    if not user:
        raise UnauthorizedAPIError("Invalid email or password")
    
    # Create tokens
    expires_delta = timedelta(days=30) if login_data.get('remember_me') else timedelta(hours=1)
    access_token = create_access_token(identity=user.id, expires_delta=expires_delta)
    refresh_token = create_refresh_token(identity=user.id)
    
    # Serialize user data
    user_schema = UserResponseSchema()
    user_response = user_schema.dump(user.to_dict())
    
    response = {
        'message': 'Login successful',
        'user': user_response,
        'tokens': {
            'access_token': access_token,
            'refresh_token': refresh_token
        }
    }
    
    logger.info(f"User logged in: {user.email}")
    return jsonify(response), 200


@api_bp.route('/auth/refresh', methods=['POST'])
@jwt_required(refresh=True)
@handle_api_errors
def refresh_token():
    """
    Refresh JWT access token.
    
    ---
    post:
      tags: [Authentication]
      summary: Refresh access token
      security:
        - Bearer: []
      responses:
        200:
          description: Token refreshed successfully
        401:
          description: Invalid refresh token
    """
    current_user_id = get_jwt_identity()
    new_access_token = create_access_token(identity=current_user_id)
    
    response = {
        'message': 'Token refreshed successfully',
        'access_token': new_access_token
    }
    
    return jsonify(response), 200


@api_bp.route('/users/profile', methods=['GET'])
@jwt_required()
@handle_api_errors
def get_user_profile():
    """
    Get current user's profile.
    
    ---
    get:
      tags: [Users]
      summary: Get user profile
      security:
        - Bearer: []
      responses:
        200:
          description: User profile retrieved successfully
        401:
          description: Unauthorized
        404:
          description: User not found
    """
    current_user_id = get_jwt_identity()
    
    # In real implementation: user = User.query.get(current_user_id)
    # For this example, create a mock user
    user = User(
        email="test@example.com",
        password="TestPassword123!",
        first_name="Test",
        last_name="User"
    )
    user.id = current_user_id
    
    if not user or user.is_deleted():
        raise NotFoundAPIError("User not found")
    
    user_schema = UserResponseSchema()
    user_response = user_schema.dump(user.to_dict())
    
    response = {
        'user': user_response
    }
    
    return jsonify(response), 200


@api_bp.route('/users/profile', methods=['PUT'])
@jwt_required()
@handle_api_errors
@validate_json_request(UserUpdateSchema)
def update_user_profile():
    """
    Update current user's profile.
    
    ---
    put:
      tags: [Users]
      summary: Update user profile
      security:
        - Bearer: []
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/UserUpdateSchema'
      responses:
        200:
          description: Profile updated successfully
        400:
          description: Validation error
        401:
          description: Unauthorized
        404:
          description: User not found
    """
    current_user_id = get_jwt_identity()
    update_data = g.validated_data
    
    # In real implementation: user = User.query.get(current_user_id)
    user = User(
        email="test@example.com",
        password="TestPassword123!",
        first_name="Test",
        last_name="User"
    )
    user.id = current_user_id
    
    if not user or user.is_deleted():
        raise NotFoundAPIError("User not found")
    
    # Update user profile
    UserService.update_user_profile(user, update_data)
    
    # In real implementation: db.session.commit()
    
    user_schema = UserResponseSchema()
    user_response = user_schema.dump(user.to_dict())
    
    response = {
        'message': 'Profile updated successfully',
        'user': user_response
    }
    
    logger.info(f"User profile updated: {user.email}")
    return jsonify(response), 200


@api_bp.route('/users', methods=['GET'])
@jwt_required()
@admin_required
@handle_api_errors
def list_users():
    """
    List all users (admin only).
    
    ---
    get:
      tags: [Users]
      summary: List all users
      security:
        - Bearer: []
      parameters:
        - name: page
          in: query
          schema:
            type: integer
            minimum: 1
            default: 1
        - name: per_page
          in: query
          schema:
            type: integer
            minimum: 1
            maximum: 100
            default: 20
        - name: search
          in: query
          schema:
            type: string
      responses:
        200:
          description: Users retrieved successfully
        401:
          description: Unauthorized
        403:
          description: Admin privileges required
    """
    # Parse query parameters
    page = request.args.get('page', 1, type=int)
    per_page = min(request.args.get('per_page', 20, type=int), 100)
    search = request.args.get('search', '', type=str)
    
    # In real implementation: query users from database with pagination
    # users = User.query.filter(User.deleted_at.is_(None))
    # if search:
    #     users = users.filter(User.email.contains(search) | 
    #                         User.first_name.contains(search) |
    #                         User.last_name.contains(search))
    # pagination = users.paginate(page=page, per_page=per_page)
    
    # Mock response
    mock_users = [
        User(email=f"user{i}@example.com", password="password", 
             first_name=f"User{i}", last_name="Test")
        for i in range(1, 6)
    ]
    
    user_schema = UserResponseSchema(many=True)
    users_response = user_schema.dump([user.to_dict() for user in mock_users])
    
    response = {
        'users': users_response,
        'pagination': {
            'page': page,
            'per_page': per_page,
            'total': len(mock_users),
            'pages': 1
        }
    }
    
    return jsonify(response), 200


@api_bp.route('/users/<int:user_id>', methods=['DELETE'])
@jwt_required()
@admin_required
@handle_api_errors
def delete_user(user_id: int):
    """
    Soft delete a user (admin only).
    
    ---
    delete:
      tags: [Users]
      summary: Delete a user
      security:
        - Bearer: []
      parameters:
        - name: user_id
          in: path
          required: true
          schema:
            type: integer
      responses:
        200:
          description: User deleted successfully
        401:
          description: Unauthorized
        403:
          description: Admin privileges required
        404:
          description: User not found
    """
    # In real implementation: user = User.query.get(user_id)
    if user_id <= 0:
        raise NotFoundAPIError("User not found")
    
    # Mock user
    user = User(
        email="user@example.com",
        password="password",
        first_name="User",
        last_name="Test"
    )
    user.id = user_id
    
    if user.is_deleted():
        raise NotFoundAPIError("User not found")
    
    # Soft delete
    user.soft_delete()
    
    # In real implementation: db.session.commit()
    
    response = {
        'message': 'User deleted successfully'
    }
    
    logger.info(f"User deleted: {user.email} (ID: {user_id})")
    return jsonify(response), 200


@api_bp.route('/health', methods=['GET'])
def health_check():
    """
    Health check endpoint.
    
    ---
    get:
      tags: [Health]
      summary: Health check
      responses:
        200:
          description: Service is healthy
    """
    return jsonify({
        'status': 'healthy',
        'service': 'user-api',
        'timestamp': datetime.utcnow().isoformat(),
        'version': '1.0.0'
    }), 200


# Error handlers

@api_bp.errorhandler(404)
def not_found(error):
    """Handle 404 errors."""
    return jsonify({
        'error': {
            'message': 'Endpoint not found',
            'code': 404
        }
    }), 404


@api_bp.errorhandler(405)
def method_not_allowed(error):
    """Handle 405 errors."""
    return jsonify({
        'error': {
            'message': 'Method not allowed',
            'code': 405
        }
    }), 405


@api_bp.errorhandler(500)
def internal_error(error):
    """Handle 500 errors."""
    logger.exception("Internal server error")
    return jsonify({
        'error': {
            'message': 'Internal server error',
            'code': 500
        }
    }), 500


# Request/Response middleware

@api_bp.before_request
def log_request_info():
    """Log request information."""
    logger.info(f"API Request: {request.method} {request.path} from {request.remote_addr}")


@api_bp.after_request
def log_response_info(response):
    """Log response information."""
    logger.info(f"API Response: {response.status_code} for {request.method} {request.path}")
    return response


# Utility functions

def create_api_response(data: Any = None, message: str = None, status_code: int = 200) -> Tuple[Dict, int]:
    """
    Create a standardized API response.
    
    Args:
        data: Response data
        message: Response message
        status_code: HTTP status code
        
    Returns:
        Tuple of (response_dict, status_code)
    """
    response = {}
    
    if message:
        response['message'] = message
    
    if data is not None:
        response['data'] = data
    
    return response, status_code


def paginate_query(query, page: int, per_page: int) -> Dict[str, Any]:
    """
    Paginate a database query.
    
    Args:
        query: SQLAlchemy query object
        page: Page number
        per_page: Items per page
        
    Returns:
        Dictionary with pagination info
    """
    # In real implementation:
    # pagination = query.paginate(page=page, per_page=per_page, error_out=False)
    # return {
    #     'items': pagination.items,
    #     'total': pagination.total,
    #     'pages': pagination.pages,
    #     'page': page,
    #     'per_page': per_page,
    #     'has_prev': pagination.has_prev,
    #     'has_next': pagination.has_next
    # }
    
    # Mock implementation
    return {
        'items': [],
        'total': 0,
        'pages': 0,
        'page': page,
        'per_page': per_page,
        'has_prev': False,
        'has_next': False
    }