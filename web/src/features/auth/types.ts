export interface User {
  username: string
  isAdmin: boolean
}

export interface AuthContext {
  user: User | null
  accessToken: string | null
}
