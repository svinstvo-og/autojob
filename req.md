this is a self-hosted personal helper service to automate find open job postings. User applies their base CV, which contains true information (CV is put to the server manually, as it is personal project, anyone , whose telegram id is on whitelist could access it, so only my own accounts). Then I can look at open jobs and generate tailored CV (and maybe cover letter) for the position (locally there is a gemini CLI, so I will query a skill for generating tailored CVs, Ill provide job posting details)
# FR
- user can query open jobs by city and speciality
- user can check new (unseen) jobs in the city and speciality
- user can tailor CV for specific position with LLM
# NFR
- job data is queried from freehire.me public API and keys seen are written to postgresql db
- city and specialities are set by user in tg bot and stored in postgresql db
- locally there is a 