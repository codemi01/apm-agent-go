module go.elastic.co/apm/module/apmiris

require (
	github.com/kataras/iris v11.1.1+incompatible
	go.elastic.co/apm v1.7.2
	go.elastic.co/apm/module/apmhttp v1.7.2
)

replace go.elastic.co/apm => ../..

replace go.elastic.co/apm/module/apmhttp => ../apmhttp

go 1.13