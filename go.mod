module github.com/martianoff/gala-server

go 1.25.5

require (
	martianoff/gala/collection_immutable v0.0.0
	martianoff/gala/concurrent v0.0.0
	martianoff/gala/go_interop v0.0.0
	martianoff/gala/json v0.0.0
	martianoff/gala/std v0.0.0
	martianoff/gala/test v0.0.0
)

replace martianoff/gala/std => C:/Users/maxmr/.gala/stdlib/v0.20.0/std

replace martianoff/gala/go_interop => C:/Users/maxmr/.gala/stdlib/v0.20.0/go_interop

replace martianoff/gala/collection_immutable => C:/Users/maxmr/.gala/stdlib/v0.20.0/collection_immutable

replace martianoff/gala/collection_mutable => C:/Users/maxmr/.gala/stdlib/v0.20.0/collection_mutable

replace martianoff/gala/concurrent => C:/Users/maxmr/.gala/stdlib/v0.20.0/concurrent

replace martianoff/gala/json => C:/Users/maxmr/.gala/stdlib/v0.20.0/json

replace martianoff/gala/test => C:/Users/maxmr/.gala/stdlib/v0.20.0/test