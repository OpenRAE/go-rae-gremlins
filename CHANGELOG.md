# Changelog

## [0.7.0](https://github.com/OpenRAE/go-rae-gremlins/compare/v0.6.0...v0.7.0) (2026-11-10)


### Features

* add output mutation diff ([#276](https://github.com/OpenRAE/go-rae-gremlins/issues/276)) ([b48a4aa](https://github.com/OpenRAE/go-rae-gremlins/commit/b48a4aad17eff33dc7262e0d1487b1a4d6eec322))
* enforce architecture model and design checks ([bb2a9b4](https://github.com/OpenRAE/go-rae-gremlins/commit/bb2a9b44a8237ce0db2c6d78cafa030b8fd71b60))
* enforce architecture model and design checks ([14ebb7f](https://github.com/OpenRAE/go-rae-gremlins/commit/14ebb7f5b624803927cb296f624b6e12efa372fe))


### Bug Fixes

* **architecture:** record design for issue 112 threshold fix ([294a75b](https://github.com/OpenRAE/go-rae-gremlins/commit/294a75b4a19850a3313e85a2629cf67527080ba1))
* **ci:** retain test identity across the fork module migration ([6ad5421](https://github.com/OpenRAE/go-rae-gremlins/commit/6ad54217d83a47997331d27c4345fa96fc35b0f1))
* clear the govulncheck and goconst findings ([9f7cc02](https://github.com/OpenRAE/go-rae-gremlins/commit/9f7cc024e1bb990f4add4c5eca35bd3637a334cb))
* **configuration:** apply unleash threshold flags ([37dad77](https://github.com/OpenRAE/go-rae-gremlins/commit/37dad77f5b14e46854a8b310851d01b87ec1dbdb))
* **coverage:** keep slash-separated names on Windows ([03e5c68](https://github.com/OpenRAE/go-rae-gremlins/commit/03e5c68e3f9ee0adb2e63e98751d1e27ee2fef8d)), closes [#12](https://github.com/OpenRAE/go-rae-gremlins/issues/12)
* **coverage:** keep the full, slash-separated file names in the coverage profile ([b72ab35](https://github.com/OpenRAE/go-rae-gremlins/commit/b72ab35ca67bb36f8b2051619980abb6eaaf89ed))
* **coverage:** remove only the leading module path from profile names ([74a712d](https://github.com/OpenRAE/go-rae-gremlins/commit/74a712d2640d55504f6116783333a417eb90760d)), closes [#57](https://github.com/OpenRAE/go-rae-gremlins/issues/57)
* **diff:** avoid a missing-else code smell in changesInFragment ([4fa1407](https://github.com/OpenRAE/go-rae-gremlins/commit/4fa14075333ec68516a23ee0f2ad4c97cac37c19))
* **diff:** select each added line within a diff hunk ([a173a9a](https://github.com/OpenRAE/go-rae-gremlins/commit/a173a9addaf07be170b879c38e5f052d65132214))
* **diff:** select each added line within a diff hunk ([a5e27f5](https://github.com/OpenRAE/go-rae-gremlins/commit/a5e27f5c20c32340ca152f48646d4829d60d4e0a))
* **engine:** classify a mutant's test run from go test output, not exit code ([63137d0](https://github.com/OpenRAE/go-rae-gremlins/commit/63137d068a5a6de6605c86c8de6a3e2eba071c14))
* **engine:** classify a mutant's test run from go test's output, not its exit code ([a2382ed](https://github.com/OpenRAE/go-rae-gremlins/commit/a2382edad7869ad1497445621ce28e8c37208174))
* **engine:** exclude testdata, vendor, and nested module trees from mutation discovery ([c5040a1](https://github.com/OpenRAE/go-rae-gremlins/commit/c5040a1cf2cb54aa0fe9d7306718d1e6a2f77e4e))
* **engine:** exclude testdata, vendor, and nested module trees from mutation discovery ([7ead1fc](https://github.com/OpenRAE/go-rae-gremlins/commit/7ead1fc3a17dad1aeb219daca6472ea6ebcfeddf))
* **engine:** pass -cpu flag as separate exec argument ([#273](https://github.com/OpenRAE/go-rae-gremlins/issues/273)) ([2cd4000](https://github.com/OpenRAE/go-rae-gremlins/commit/2cd4000817d7886fedeecd151715b6de1d45d34b))
* **engine:** prevent orphaned process accumulation during mutation testing ([#260](https://github.com/OpenRAE/go-rae-gremlins/issues/260)) ([9e7bdf2](https://github.com/OpenRAE/go-rae-gremlins/commit/9e7bdf239eb07bddb6a953c641e6b27006d344b3))
* **engine:** read evaluated switch and select clause expressions as covered ([c0740e1](https://github.com/OpenRAE/go-rae-gremlins/commit/c0740e147c36d50d990deda59c18219117fbf2db))
* **engine:** read evaluated switch and select clause expressions as covered ([c5857a4](https://github.com/OpenRAE/go-rae-gremlins/commit/c5857a41302439c4b2d2fdc10b7f3b0a0fcb22c0))
* **engine:** run the tests of the mutated file's package directory ([c3ff44c](https://github.com/OpenRAE/go-rae-gremlins/commit/c3ff44c93c7ea1f46385414ee58b3e70abeb7647))
* **engine:** run the tests of the mutated file's package directory ([afeca14](https://github.com/OpenRAE/go-rae-gremlins/commit/afeca143243cbe80ffc545c38788b9fd7db5ac18))
* make the fork ready for a release that dependents can pin ([425a996](https://github.com/OpenRAE/go-rae-gremlins/commit/425a9967b0c59ed697e6761312386c55296c651d))
* preserve Windows tool cache environment paths ([d5763a6](https://github.com/OpenRAE/go-rae-gremlins/commit/d5763a60eeee58d7d78ae9a05fe9299c18352cce))
* **release:** make the release work with goreleaser v2 and document it ([9803d54](https://github.com/OpenRAE/go-rae-gremlins/commit/9803d54788ba6edbd099b627c218061a4ec8c3b9))
* **release:** make the release work with goreleaser v2 and document it ([d8864b9](https://github.com/OpenRAE/go-rae-gremlins/commit/d8864b9d8e301e221904522c70d91a4f283aadda))
* report the module version of go install builds and remove release blocks ([c467314](https://github.com/OpenRAE/go-rae-gremlins/commit/c4673142f2ea2c430bdf2da74421d95108d6a30a))
* **report:** accept a result equal to its minimum threshold ([3d24c2e](https://github.com/OpenRAE/go-rae-gremlins/commit/3d24c2ec478602f6a52b9ac9ea9c1cbe15c8332a))
* **report:** accept a result equal to its minimum threshold ([d3f8b35](https://github.com/OpenRAE/go-rae-gremlins/commit/d3f8b35211fa516767f1efe4558519aaaceaa75f))
* resolve checkout aliases when checking documentation links ([3088df0](https://github.com/OpenRAE/go-rae-gremlins/commit/3088df033ffc51c655ea08adb83f44a38414c70a))
* satisfy promoted lint and regression classification checks ([becde47](https://github.com/OpenRAE/go-rae-gremlins/commit/becde47e6ab2643a55ecde69b758d8ea116aea36))
* validate reconciled threshold tests and policy target ([7fed3a1](https://github.com/OpenRAE/go-rae-gremlins/commit/7fed3a1507ddff325edd2206eb406afcdea53b37))
