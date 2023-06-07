(() => {
    const config = {
        cache: null,
        stage: false,
        server: "example.com",
        ttl: 3600,
        score: 0.9,
        dns: ["1.1.1.1", "8.8.8.8"],
        paths: {
            "home": "/",
            "blog": "/blog"
        }
    }
})()
