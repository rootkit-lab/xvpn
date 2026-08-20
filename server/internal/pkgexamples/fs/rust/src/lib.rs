pub fn greet(name: &str) -> String {
    let n = if name.is_empty() { "xgit" } else { name };
    format!("hello {n}")
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn greet_ok() {
        assert_eq!(greet("rs"), "hello rs");
    }
}
