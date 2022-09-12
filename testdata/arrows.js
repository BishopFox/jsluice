let hello = () => {
    console.log("Hello")

    let val1 = "val1"
    let val2 = "val2"

    fetch("/api/example?param1\x3d"+val1+"&param2="+val2, {
        "method": "POST",
        credentials: "include"
    })

    fetch("/api/example2?param3="+val2+"&param4\u003d"+val2, {
        method: "PUT"
    })

    fetch("/api/example3?param3\075"+val1+"&param3\u{00003D}"+val2, {headers: {'Accept': "application/json"}})
}
