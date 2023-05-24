function goToLogin(){
    location.href = "/login/" + document.location.hash.substring(1)
}
let logout = () => {
    document.location.replace("/logout")
}
