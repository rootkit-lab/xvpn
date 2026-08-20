package com.ihuull;

public final class Hello {
  private Hello() {}

  public static String greet(String name) {
    return "hello " + name;
  }

  public static void main(String[] args) {
    System.out.println(greet("mvn"));
  }
}
