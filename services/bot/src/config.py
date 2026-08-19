from pydantic_settings import BaseSettings, SettingsConfigDict

class Settings(BaseSettings):
    bot_token: str
    generator_url: str = "http://localhost:8000"
    
    model_config = SettingsConfigDict(env_file=".env")

settings = Settings()
