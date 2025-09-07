from typing import Optional

class ConnectionPool:
    pass

class Config:
    # Simple assignment
    DEFAULT_TIMEOUT = 30
    
    # Type annotation with assignment
    API_VERSION: str = "v1"
    
    # Type annotation only
    connection_pool: Optional[ConnectionPool] = None