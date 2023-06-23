$.ajax({
    method: "PUT",
    url: "/api/v1/posts",
    data:{ postId: 324 },
    headers: {
        "Content-Type": "application/json",
        "x-backend": "prod"
    }},
    function(data, status){
        location.href = data.redirect;
    }
)

