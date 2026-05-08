export interface User {
  id: string
  username: string
  isAdmin: boolean
}

export interface AuthContext {
  user: User | null
  accessToken: string | null
}
