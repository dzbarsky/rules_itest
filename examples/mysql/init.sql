/* The init.sql mechanism exists for ultra-fast DB setup. How to use it: */
/* 1. Start with a fresh DB and get it to a state you are happy with. You can write a script that sets things up the way you want (perhaps runs migrations normally, or sets up some test scenarios, etc.) */
/* 2. Run `mysqldump --skip-extended-inserts --skip-comments --databases your,databases,go,here --flush-privileges > init.sql` or similar */
/* 3. Your tests can use an `itest_service` wrapped around a `mysql_impl` like the one in `//examples/mysql:with_migrations`; mysql will come up with the init.sql already applied. */

/** Example **/
CREATE USER 'user'@'localhost' IDENTIFIED BY 'password';