"""
Flask web application for user management
"""

from flask import Flask, request, jsonify
from dataclasses import dataclass
from typing import List, Optional
import logging

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

@dataclass
class User:
    """User data model"""
    id: int
    name: str
    email: str
    active: bool = True

class UserService:
    """Service layer for user operations"""
    
    def __init__(self):
        self.users: List[User] = [
            User(1, "Alice Johnson", "alice@example.com"),
            User(2, "Bob Smith", "bob@example.com"),
            User(3, "Carol Davis", "carol@example.com"),
        ]
        self.next_id = 4
    
    def get_all_users(self) -> List[User]:
        """Retrieve all active users"""
        return [user for user in self.users if user.active]
    
    def get_user_by_id(self, user_id: int) -> Optional[User]:
        """Retrieve a specific user by ID"""
        for user in self.users:
            if user.id == user_id and user.active:
                return user
        return None
    
    def create_user(self, name: str, email: str) -> User:
        """Create a new user"""
        user = User(self.next_id, name, email)
        self.users.append(user)
        self.next_id += 1
        return user
    
    def update_user(self, user_id: int, name: str = None, email: str = None) -> Optional[User]:
        """Update an existing user"""
        user = self.get_user_by_id(user_id)
        if not user:
            return None
        
        if name is not None:
            user.name = name
        if email is not None:
            user.email = email
        
        return user
    
    def delete_user(self, user_id: int) -> bool:
        """Soft delete a user"""
        user = self.get_user_by_id(user_id)
        if user:
            user.active = False
            return True
        return False

# Initialize Flask app and services
app = Flask(__name__)
user_service = UserService()

@app.route('/health', methods=['GET'])
def health_check():
    """Health check endpoint"""
    return jsonify({'status': 'healthy', 'service': 'user-management'})

@app.route('/api/users', methods=['GET'])
def get_users():
    """Get all users endpoint"""
    try:
        users = user_service.get_all_users()
        return jsonify({
            'users': [user.__dict__ for user in users],
            'total': len(users)
        })
    except Exception as e:
        logger.error(f"Error retrieving users: {e}")
        return jsonify({'error': 'Internal server error'}), 500

@app.route('/api/users/<int:user_id>', methods=['GET'])
def get_user(user_id: int):
    """Get specific user endpoint"""
    try:
        user = user_service.get_user_by_id(user_id)
        if not user:
            return jsonify({'error': 'User not found'}), 404
        
        return jsonify({'user': user.__dict__})
    except Exception as e:
        logger.error(f"Error retrieving user {user_id}: {e}")
        return jsonify({'error': 'Internal server error'}), 500

@app.route('/api/users', methods=['POST'])
def create_user():
    """Create new user endpoint"""
    try:
        data = request.get_json()
        
        if not data or 'name' not in data or 'email' not in data:
            return jsonify({'error': 'Name and email are required'}), 400
        
        user = user_service.create_user(data['name'], data['email'])
        return jsonify({
            'message': 'User created successfully',
            'user': user.__dict__
        }), 201
    
    except Exception as e:
        logger.error(f"Error creating user: {e}")
        return jsonify({'error': 'Internal server error'}), 500

@app.route('/api/users/<int:user_id>', methods=['PUT'])
def update_user(user_id: int):
    """Update user endpoint"""
    try:
        data = request.get_json()
        if not data:
            return jsonify({'error': 'No data provided'}), 400
        
        user = user_service.update_user(
            user_id, 
            data.get('name'), 
            data.get('email')
        )
        
        if not user:
            return jsonify({'error': 'User not found'}), 404
        
        return jsonify({
            'message': 'User updated successfully',
            'user': user.__dict__
        })
    
    except Exception as e:
        logger.error(f"Error updating user {user_id}: {e}")
        return jsonify({'error': 'Internal server error'}), 500

@app.route('/api/users/<int:user_id>', methods=['DELETE'])
def delete_user(user_id: int):
    """Delete user endpoint"""
    try:
        if user_service.delete_user(user_id):
            return jsonify({'message': 'User deleted successfully'})
        else:
            return jsonify({'error': 'User not found'}), 404
    
    except Exception as e:
        logger.error(f"Error deleting user {user_id}: {e}")
        return jsonify({'error': 'Internal server error'}), 500

@app.errorhandler(404)
def not_found(error):
    """404 error handler"""
    return jsonify({'error': 'Resource not found'}), 404

@app.errorhandler(500)
def internal_error(error):
    """500 error handler"""
    logger.error(f"Internal server error: {error}")
    return jsonify({'error': 'Internal server error'}), 500

if __name__ == '__main__':
    app.run(debug=True, host='0.0.0.0', port=5000)